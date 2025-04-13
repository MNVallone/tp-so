package globales

import (
	"encoding/json"
	"os"
	"log"
	"strings"
	"bufio"
	"net/http"
	"io"
	"bytes"
	"fmt"
)

type Paquete struct {
	Valores string   `json:"valores"`
}

// ------ LOGGING ------ //
func ConfigurarLogger(nombreArchivoLog string) {
	logFile, err := os.OpenFile(nombreArchivoLog, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		log.Println("No se pudo crear el logger")
		panic(err)
	}

	// MultiWriter: escribe en consola y archivo a la vez
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)

	log.Println("Logger iniciado correctamente")
}


func LeerConsola() strings.Builder {
	var buffer strings.Builder
	// Leer de la consola
	reader := bufio.NewReader(os.Stdin)
	log.Println("Ingrese los mensajes")
	
	for text, _ := reader.ReadString('\n'); text != "\n"; {
		buffer.WriteString(text)
		text, _ = reader.ReadString('\n')
	}

	return buffer
}

// ------ PAQUETE ------ //
func GenerarYEnviarPaquete[T any](estructura *T, ip string, puerto int) {
	// URL del servidor 
	url := fmt.Sprintf("http://%s:%d/paquete", ip, puerto)


	// Converir el paquete a formato JSON
	body, err := json.Marshal(estructura)
	if err != nil {
		log.Printf("Error codificando el paquete: %s", err.Error())
		return
	}

	// Enviamos el POST al servidor
	byteData := []byte(body) // castearlo a bytes antes de enviarlo
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(byteData))
	if err != nil {
		log.Printf("Error enviando mensajes a ip:%s puerto:%d", ip, puerto)
		return
	}
	defer resp.Body.Close()

	// Verificar respuesta del servidor
	if resp.StatusCode != http.StatusOK {
		log.Printf("Error en la respuesta del servidor: %s", resp.Status)
		return
	}
	log.Printf("Respuesta del servidor: %s", resp.Status)

	log.Printf("Paquete enviado!")
}
