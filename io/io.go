package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"globales"
	"log"
	"net/http"
	"github.com/sisoputnfrba/tp-golang/io/utils"
)

type Paquete struct {
	Valores []string `json:"valores"`
}

func main() {
	// ------ LOGGING ------ //
	globales.ConfigurarLogger("io.log") // configurar logger

	// ------ CONFIGURACIONES ------ //
	utils.ClientConfig = utils.IniciarConfiguracion("config.json")
	if utils.ClientConfig == nil {
		log.Fatalf("No se pudo crear el config")
	}

	/*
	paquete := Paquete{}
	paquete.Valores = append(paquete.Valores, "Un mensaje para kernel")
	body, err := json.Marshal(paquete)

	if err != nil {
		log.Printf("Error codificando el paquete: %s", err.Error())
		return
	}

	// URL del servidor para enviar el paquete
	url := fmt.Sprintf("http://localhost:8001/paquete")

	// Enviamos el POST al servidor
	byteData := []byte(body) // castearlo a bytes antes de enviarlo
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(byteData))
	if err != nil {
		log.Printf("Error enviando mensajes a puerto: 8001")
		return
	}
	defer resp.Body.Close()

	// Verificar respuesta del servidor
	if resp.StatusCode != http.StatusOK {
		log.Printf("Error en la respuesta del servidor: %s", resp.Status)
		return
	}

	log.Printf("Respuesta del servidor: %s", resp.Status)
	*/
}
