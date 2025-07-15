package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
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
	http.HandleFunc("/clima", climaHandler)
	fmt.Println("Servidor rodando na porta 8080...")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	http.ListenAndServe(":"+port, nil)
	// http.ListenAndServe(":8080", nil)
}

var (
	viaCEPurlBase     = "https://viacep.com.br/ws/"
	weatherAPIurlBase = "https://api.weatherapi.com/v1/current.json"
)

func climaHandler(w http.ResponseWriter, r *http.Request) {
	cep := r.URL.Query().Get("cep")
	apiKey := r.URL.Query().Get("apiKey")

	// Validação do CEP (sempre 8 dígitos numéricos)
	if !regexp.MustCompile(`^\d{8}$`).MatchString(cep) {
		http.Error(w, `{"message": "invalid zipcode"}`, http.StatusUnprocessableEntity)
		return
	}

	// Consulta ViaCEP
	// viaCEPurl := fmt.Sprintf("https://viacep.com.br/ws/%s/json/", cep)
	viaCEPurl := fmt.Sprintf("%s%s/json/", viaCEPurlBase, cep)
	resp, err := http.Get(viaCEPurl)
	if err != nil {
		http.Error(w, `{"message": "erro ao consultar ViaCEP"}`, http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, `{"message": "can not find zipcode"}`, http.StatusNotFound)
		return
	}

	body, _ := io.ReadAll(resp.Body)
	var cepData ViaCEPResponse

	if err := json.Unmarshal(body, &cepData); err != nil {
		http.Error(w, `{"message": "erro ao processar resposta do ViaCEP"}`, http.StatusInternalServerError)
		return
	}

	if cepData.isErro() || cepData.Localidade == "" {
		http.Error(w, `{"message": "can not find zipcode"}`, http.StatusNotFound)
		return
	}

	// Consulta WeatherAPI
	// weatherAPIurl := fmt.Sprintf("https://api.weatherapi.com/v1/current.json?key=%s&q=%s", apiKey, cepData.Localidade)
	weatherAPIurl := fmt.Sprintf("%s?key=%s&q=%s", weatherAPIurlBase, apiKey, cepData.Localidade)
	resp, err = http.Get(weatherAPIurl)
	if err != nil {
		http.Error(w, `{"message": "erro ao consultar WeatherAPI"}`, http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, _ = io.ReadAll(resp.Body)
	var weatherData WeatherAPIResponse
	if err := json.Unmarshal(body, &weatherData); err != nil {
		http.Error(w, `{"message": "erro ao processar dados do clima"}`, http.StatusInternalServerError)
		return
	}

	tempC := weatherData.Current.TempC
	tempF := tempC*1.8 + 32 // Conversão de Celsius para Fahrenheit
	tempK := tempC + 273    // Conversão de Celsius para Kelvin

	// Resposta JSON
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]float64{
		"tempC": tempC,
		"tempF": tempF,
		"tempK": tempK,
	})
}
