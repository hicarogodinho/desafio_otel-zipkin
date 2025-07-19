package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

type CEPRequest struct {
	Cep string `json:"cep"`
}

func main() {
	initTracer()

	mux := http.NewServeMux()
	mux.Handle("/consulta", otelhttp.NewHandler(http.HandlerFunc(cepHandler), "cepHandler"))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	fmt.Printf("Serviço A rodando na porta %s...\n", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func cepHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req CEPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"message": "invalid request"}`, http.StatusBadRequest)
		return
	}

	if !regexp.MustCompile(`^\d{8}$`).MatchString(req.Cep) {
		http.Error(w, `{"message": "invalid zipcode"}`, http.StatusUnprocessableEntity)
		return
	}

	// Cria um span para a chamada ao serviço B
	tracer := otel.Tracer("servico_a")
	ctx, span := tracer.Start(ctx, "Chamada ao serviço B")
	defer span.End()

	// Encaminha a solicitação para o serviço B
	servicoBURL := fmt.Sprintf("http://servico_b:8080/clima?cep=%s&apiKey=ff4e9f3ecf62466396a141841251407", req.Cep)
	reqB, _ := http.NewRequestWithContext(ctx, "GET", servicoBURL, nil)

	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(reqB.Header))

	client := http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}
	resp, err := client.Do(reqB)
	if err != nil {
		log.Printf("Erro ao chamar serviço B: %v", err)
		http.Error(w, `{"message": "erro ao consultar o serviço B"}`, http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Erro ao ler resposta do serviço B: %v", err)
		http.Error(w, `{"message": "erro ao processar resposta do serviço B"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
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
			semconv.ServiceName("servico_a"),
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

	otel.SetTextMapPropagator(propagation.TraceContext{})

	go func() {
		<-ctx.Done()
		_ = tp.Shutdown(context.Background())
	}()
}
