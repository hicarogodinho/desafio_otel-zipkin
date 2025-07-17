package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

type ViaCEPResponse struct {
	Localidade string          `json:"localidade"`
	Estado     string          `json:"estado"`
	Erro       json.RawMessage `json:"erro,omitempty"`
}

func (v ViaCEPResponse) isErro() bool {
	if v.Erro == nil {
		return false
	}

	var b bool
	if err := json.Unmarshal(v.Erro, &b); err == nil {
		return b
	}

	return false
}

type WeatherAPIResponse struct {
	Current struct {
		TempC float64 `json:"temp_c"`
	} `json:"current"`
}

func main() {
	initTracer()

	mux := http.NewServeMux()
	mux.Handle("/clima", otelhttp.NewHandler(http.HandlerFunc(climaHandler), "climaHandler"))
	// http.HandleFunc("/clima", climaHandler)
	// fmt.Println("Servidor rodando na porta 8080...")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Println("Serviço B rodando na porta ", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
	// http.ListenAndServe(":8080", nil)
}

var (
	viaCEPurlBase     = "https://viacep.com.br/ws/"
	weatherAPIurlBase = "https://api.weatherapi.com/v1/current.json"
)

func climaHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tr := otel.Tracer("servico_b")

	cep := r.URL.Query().Get("cep")
	apiKey := r.URL.Query().Get("apiKey")

	// Validação do CEP (sempre 8 dígitos numéricos)
	if !regexp.MustCompile(`^\d{8}$`).MatchString(cep) {
		http.Error(w, `{"message": "invalid zipcode"}`, http.StatusUnprocessableEntity)
		return
	}

	var cepData ViaCEPResponse
	var weatherData WeatherAPIResponse

	// Span para ViaCEP
	err := func(ctx context.Context) error {
		ctx, span := tr.Start(ctx, "Chamada ao ViaCEP")
		defer span.End()

		viaCEPurl := fmt.Sprintf("%s%s/json/", viaCEPurlBase, cep)
		resp, err := http.Get(viaCEPurl)
		if err != nil {
			return fmt.Errorf("erro ao consultar ViaCEP")
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			http.Error(w, `{"message": "can not find zipcode"}`, http.StatusNotFound)
		}

		body, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(body, &cepData); err != nil {
			return fmt.Errorf("erro ao processar resposta do ViaCEP: %w", err)
		}
		return nil
	}(ctx)
	if err != nil {
		http.Error(w, `{"message": "erro ao consultar ViaCEP"}`, http.StatusInternalServerError)
		return
	}

	// Span para WeatherAPI
	err = func(ctx context.Context) error {
		ctx, span := tr.Start(ctx, "Chamada ao WeatherAPI")
		defer span.End()

		escapedCity := url.QueryEscape(cepData.Localidade)
		weatherAPIurl := fmt.Sprintf("%s?key=%s&q=%s", weatherAPIurlBase, apiKey, escapedCity)
		resp, err := http.Get(weatherAPIurl)
		if err != nil {
			return fmt.Errorf("erro ao consultar WeatherAPI")
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(body, &weatherData); err != nil {
			return fmt.Errorf("erro ao processar resposta do WeatherAPI: %w", err)
		}
		return nil
	}(ctx)
	if err != nil {
		http.Error(w, `{"message": "erro ao consultar WeatherAPI"}`, http.StatusInternalServerError)
		return
	}

	tempC := weatherData.Current.TempC
	tempF := tempC*1.8 + 32 // Conversão de Celsius para Fahrenheit
	tempK := tempC + 273    // Conversão de Celsius para Kelvin

	// Resposta JSON
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"city":   cepData.Localidade,
		"temp_C": tempC,
		"temp_F": tempF,
		"temp_K": tempK,
	})
}

func initTracer() {
	ctx := context.Background()

	// Exportador OTLP via HTTP apontando para o OTEL Collector
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint("otel-collector:4318"),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		log.Fatalf("Erro ao criar exportador OTLP: %v", err)
	}

	// Define o nome do serviço para aparecer no Zipkin
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("servico_b"),
		),
	)
	if err != nil {
		log.Fatalf("Erro ao criar recurso: %v", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
}
