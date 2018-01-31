package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

func main() {
	mw := multiWeatherProvider{
		openWeatherMap{apiKey: "x"},
		weatherUnderground{apiKey: "x"},
		darkySky{apiKey: "x"}
	}

	http.HandleFunc("/weather/", func(w http.ResponseWriter, r *http.Request) {
		begin := time.Now()
		city := strings.SplitN(r.URL.Path, "/", 3)[2]
		temp, err := mw.temperature(city)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"city": city,
			"temp": temp,
			"took": time.Since(begin).String(),
		})
	})
	http.ListenAndServe(":8080", nil)
}

type multiWeatherProvider []weatherProvider

type weatherProvider interface {
	temperature(city string) (float64, error)
}

func (w multiWeatherProvider) temperature(city string) (float64, error) {
	// Make a channel for temperatures, and a channel for errors.
	// Each provider will push a value into only one.
	temps := make(chan float64, len(w))
	errs := make(chan error, len(w))

	// For each provider, spawn a goroutine with an anonymous function.
	// That function will invoke the temperature method, and forward the response.
	for _, provider := range w {
		go func(p weatherProvider) {
			k, err := p.temperature(city)
			if err != nil {
				errs <- err
				return
			}
			temps <- k
		}(provider)
	}
	sum := 0.0
	// Collect a temperature or an error from each provider.
	for i := 0; i < len(w); i++ {
		select {
		case temp := <-temps:
			sum += temp
		case err := <-errs:
			return 0, err
		}
	}
	// Return the average, same as before.
	return sum / float64(len(w)), nil
}

func temperature(city string, providers ...weatherProvider) (float64, error) {
	sum := 0.0
	for _, provider := range providers {
		k, err := provider.temperature(city)
		if err != nil {
			return 0, err
		}
		sum += k
	}
	return sum / float64(len(providers)), nil
}

type weatherData struct {
	Name string `json:"name"`
	Main struct {
		Kelvin float64 `json:"temp"`
	} `json:"main"`
}

type openWeatherMap struct {
	apiKey string
}

func (w openWeatherMap) temperature(city string) (float64, error) {
	resp, err := http.Get("http://api.openweathermap.org/data/2.5/weather?APPID=" + w.apiKey + "&q=" + city)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var d struct {
		Main struct {
			Kelvin float64 `json:"temp"`
		} `json:"main"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return 0, err
	}
	log.Printf("openWeatherMap: %s: %.2f", city, d.Main.Kelvin)
	return d.Main.Kelvin, nil
}

type weatherUnderground struct {
	apiKey string
}

func (w weatherUnderground) temperature(city string) (float64, error) {
	resp, err := http.Get("http://api.wunderground.com/api/" + w.apiKey + "/conditions/q/" + city + ".json")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var d struct {
		Observation struct {
			Celsius float64 `json:"temp_c"`
		} `json:"current_observation"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return 0, err
	}
	kelvin := d.Observation.Celsius + 273.15
	log.Printf("weatherUnderground: %s: %.2f", city, kelvin)
	return kelvin, nil
}

func (w darkSky) temperature(city string) (float64, error) {
	resp, err := http.Get("https://api.darksky.net/forecast/" + w.apiKey + "/conditions/q/" + city)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var d struct {
		Main struct {
			Kelvin float64 `json:"temp"`
		} `json:"main"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return 0, err
	}
	log.Printf("darkSky: %s: %.2f", city, d.Main.Kelvin)
	return d.Main.Kelvin, nil
}
