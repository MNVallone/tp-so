package globales

import (
	"encoding/json"
	"os"
	"log"
	"strings"
	"globales/servidor"
	"bufio"
	"net/http"
	"io"
	"bytes"
)

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
func GenerarYEnviarPaquete() {
	paquete := servidor.Paquete{}
	// Leemos y cargamos el paquete
	buffer := LeerConsola()
	paquete.Valores = append(paquete.Valores, buffer.String())
	jsonData, _ := json.Marshal(paquete)
	byteData := []byte(jsonData)

	log.Printf("paqute a enviar: %+v", jsonData)
	// Enviamos el paqute
	r, err := http.NewRequest("POST", "http://localhost:8080/paquetes", bytes.NewBuffer(byteData))
	if err != nil {
		panic(err)
	}
	client := &http.Client{}
	client.Do(r);
}

/*
func GenerarYEnviarPaquete2(mensajes []string) {

	// Si no hay mensajes, salimos
	if len(mensajes) == 0 {
		log.Println("No se ingresaron mensajes para enviar")
		return
	}

	// Crea paquete con la clave y mensajes leidos
	paquete := Paquete{
		Ip: globals.ClientConfig.Ip,
		Puerto: globals.ClientConfig.Puerto,
		Clave: globals.ClientConfig.Clave,
		Valores: mensajes,
	}

	log.Printf("Paqute a enviar: %+v", paquete)

	// Enviamos el paquete al servidor
	EnviarPaquete(globals.ClientConfig.Ip, globals.ClientConfig.Puerto, paquete)
}

func EnviarPaquete(ip string, puerto int, paquete Paquete) {
	// Converir el paquete a formato JSON
	body, err := json.Marshal(paquete)
	if err != nil {
		log.Printf("Error codificando el paquete: %s", err.Error())
		return
	}

	// URL del servidor para enviar el paquete
	url := fmt.Sprintf("http://%s:%d/paquetes", ip, puerto)

	// Enviamos el POST al servidor
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
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
}
*/