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
	"log/slog"
)

type PCB struct{
	PID int `json:"pid"`
	PC int `json:"pc"`
	ME METRICAS_DE_ESTADO `json:"metricas_de_estado"`
	MT METRICAS_DE_TIEMPO `json:"metricas_de_tiempo"`
}

//TODO: implementar metricas de estado y metricas de tiempo
type METRICAS_DE_ESTADO struct {}

type METRICAS_DE_TIEMPO struct {}

type Paquete struct {
	Valores string   `json:"valores"`
}

// ------ LOGGING ------ //
func ConfigurarLogger(nombreArchivoLog string, log_level string) {
	logFile, err := os.OpenFile(nombreArchivoLog, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		log.Println("No se pudo crear el logger")
		panic(err)
	}

	// MultiWriter: escribe en consola y archivo a la vez
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)

	nivel := LogLevelFromString(log_level)
	
	slog.SetLogLoggerLevel(nivel)
	
	slog.Info("Logger iniciado correctamente")
}

// Nivel de log
func LogLevelFromString(nivel string) slog.Level {
	switch strings.ToUpper(nivel) {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
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
func GenerarYEnviarPaquete[T any](estructura *T, ip string, puerto int, ruta string) {
	// URL del servidor 
	url := fmt.Sprintf("http://%s:%d%s", ip, puerto, ruta)

	// Converir el paquete a formato JSON
	body, err := json.Marshal(estructura)
	if err != nil {
		slog.Error(fmt.Sprintf("Error codificando el paquete: %s", err.Error()))
		return
	}

	// Enviamos el POST al servidor
	byteData := []byte(body) // castearlo a bytes antes de enviarlo
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(byteData))
	if err != nil {
		slog.Info(fmt.Sprintf("Error enviando mensajes a ip:%s puerto:%d", ip, puerto))
		return
	}
	defer resp.Body.Close()

	// Verificar respuesta del servidor
	if resp.StatusCode != http.StatusOK {
		slog.Error(fmt.Sprintf("Error en la respuesta del servidor: %s", resp.Status))
		return
	}
	slog.Info(fmt.Sprintf("Respuesta del servidor: %s", resp.Status))

	slog.Info("Paquete enviado!")
}
