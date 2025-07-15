package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fakeViaCEPAndWeatherAPI() *httptest.Server {
	mux := http.NewServeMux()

	// ViaCEP devolve "TesteCity"
	mux.HandleFunc("/ws/", func(w http.ResponseWriter, r *http.Request) {
		// _ = r.URL.Path
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		cep := ""
		if len(parts) >= 2 {
			cep = parts[1]
		}

		if cep == "00000000" { // Simula CEP inexistente
			json.NewEncoder(w).Encode(map[string]interface{}{
				"erro": true,
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"localidade": "TesteCity",
		})
	})

	// WeatherAPI devolve "10"
	mux.HandleFunc("/v1/current.json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"current": map[string]interface{}{
				"temp_c": 10,
			},
		})
	})

	return httptest.NewServer(mux)
}

func TestClimaHandler(t *testing.T) {
	t.Run("sucesso 200", func(t *testing.T) {
		ts := fakeViaCEPAndWeatherAPI()
		defer ts.Close()

		viaCEPurlBase = ts.URL + "/ws/"
		weatherAPIurlBase = ts.URL + "/v1/current.json"

		req := httptest.NewRequest("GET", "/clima?cep=12345678&apiKey=fake", nil)
		w := httptest.NewRecorder()
		climaHandler(w, req)
		resp := w.Result()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status %d, esperado 200", resp.StatusCode)
		}

		var body map[string]float64
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("body inválido: %v", err)
		}

		want := map[string]float64{
			"tempC": 10,
			"tempF": 10*1.8 + 32,
			"tempK": 10 + 273,
		}

		for k, v := range want {
			if body[k] != v {
				t.Errorf("%s = %v, esperado %v", k, body[k], v)
			}
		}
	})

	t.Run("CEP inválido 422", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/clima?cep=12345&apiKey=fake", nil) // CEP < 8 dígitos
		w := httptest.NewRecorder()
		climaHandler(w, req)
		resp := w.Result()

		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("status %d, esperado 422", resp.StatusCode)
		}

		var body struct {
			Message string `json:"message"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("body inválido: %v", err)
		}
		if body.Message != "invalid zipcode" {
			t.Errorf("mensagem = %q, esperada %q", body.Message, "invalid zipcode")
		}
	})

	t.Run("CEP não encontrado 404", func(t *testing.T) {
		ts := fakeViaCEPAndWeatherAPI()
		defer ts.Close()

		viaCEPurlBase = ts.URL + "/ws/"
		weatherAPIurlBase = ts.URL + "/v1/current.json"

		req := httptest.NewRequest("GET", "/clima?cep=00000000&apiKey=fake", nil) // CEP inexistente
		w := httptest.NewRecorder()
		climaHandler(w, req)
		resp := w.Result()

		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status %d, esperado 404", resp.StatusCode)
		}

		var body struct{ Message string }
		_ = json.NewDecoder(resp.Body).Decode(&body)
		if body.Message != "can not find zipcode" {
			t.Errorf("mensagem = %q, esperada %q", body.Message, "can not find zipcode")
		}
	})
}
