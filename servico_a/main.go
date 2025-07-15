package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
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
	servicoBURL := fmt.Sprintf("http://localhost:8080/clima?cep=%s&apiKey=ff4e9f3ecf62466396a141841251407", req.Cep)
	reqB, _ := http.NewRequestWithContext(ctx, "GET", servicoBURL, nil)

	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(reqB.Header))

	client := http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}
	resp, err := client.Do(reqB)
	if err != nil {
		http.Error(w, `{"message": "erro ao consultar o serviço B"}`, http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

func initTracer() {
	endpoint := "http://localhost:9411/api/v2/spans"
	exporter, err := zipkin.New(endpoint)
	if err != nil {
		log.Fatalf("Failed to create Zipkin exporter: %v", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
}
