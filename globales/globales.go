package globales

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
)

// ------ ESTRUCTURAS GLOBALES ------ //

type MEMORIA_CREACION_PROCESO struct {
	PID                     int    `json:"pid"`
	RutaArchivoPseudocodigo string `json:"RutaArchivoPseudocodigo"`
	Tamanio                 int    `json:"tamanio"`
}

// CPU //
type HandshakeCPU struct {
	ID_CPU     string   `json:"id_cpu"`
	PORT_CPU   int      `json:"port_cpu"`
	IP_CPU     string   `json:"ip_cpu"`
	DISPONIBLE chan int `json:"-"`
}

type SolicitudIO struct {
	NOMBRE string `json:"nombre"`
	TIEMPO int    `json:"tiempo"` // en milisegundos
	PID    int    `json:"pid"`
	PC     int    `json:"pc"`
}

type SolicitudDump struct {
	NOMBRE string `json:"nombre"`
	TIEMPO int    `json:"tiempo"` // en milisegundos
	PID    int    `json:"pid"`
	PC     int    `json:"pc"`
}

type SolicitudProceso struct {
	ARCHIVO_PSEUDOCODIGO string `json:"archivo_pseudocodigo"`
	TAMAÑO_PROCESO       int    `json:"tamanio_proceso"`
	PID                  int    `json:"pid"`
}

type PeticionCPU struct {
	PID int `json:"pid"`
	PC  int `json:"pc"`
}

type Interrupcion struct {
	PID    int    `json:"pid"`
	PC     int    `json:"pc"`
	MOTIVO string `json:"motivo"`
}

type PID struct {
	NUMERO_PID int `json:"NumeroPID"`
}

// MEMORIA //
type LeerMemoria struct {
	DIRECCION int `json:"direccion"`
	PID       int `json:"pid"`
	TAMANIO   int `json:"tamanio"`
}

type LeerMarcoMemoria struct {
	DIRECCION int `json:"direccion"`
	PID       int `json:"pid"`
}

type EscribirMemoria struct {
	DIRECCION int    `json:"direccion"`
	PID       int    `json:"pid"`
	DATOS     string `json:"datos"`
}

type EscribirMarcoMemoria struct {
	DIRECCION int    `json:"direccion"`
	PID       int    `json:"pid"`
	DATOS     []byte `json:"datos"`
}

type ParametrosMemoria struct {
	CantidadEntradas int
	TamanioPagina    int
	CantidadNiveles  int
}

type ObtenerMarco struct {
	PID              int   `json:"pid"`
	Entradas_Nivel_X []int `json:"entradas_nivel_x"` // Representa las entradas de la tabla de páginas
}

type LeerPaginaCompleta struct {
	PID        int `json:"pid"`
	DIR_FISICA int `json:"dir_fisica"`
}

type DestruirProceso struct {
	PID int `json:"pid"`
}

type PIDAEliminar struct {
	NUMERO_PID int `json:"numero_pid"`
	TAMANIO    int `json:"tamanio"`
}

// Revisando la consigna nos dimos cuenta que no nos piden interactuar con los registros del CPU
// PC va a ser una variable propia de cada instancia del modulo CPU.

type PeticionInstruccion struct {
	PC  int `json:"pc"`
	PID int `json:"pid"`
}

// ------ FUNCIONES GLOBALES ------ //
// Logging
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

// LeerConsola: lee la consola hasta que se ingrese una línea vacía
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

// Enviar paquetes de cualquier tipo
func GenerarYEnviarPaquete[T any](estructura *T, ip string, puerto int, ruta string) (*http.Response, []byte) {
	// URL del servidor
	url := fmt.Sprintf("http://%s:%d%s", ip, puerto, ruta)

	// Converir el paquete a formato JSON
	body, err := json.Marshal(estructura)
	if err != nil {
		slog.Error(fmt.Sprintf("Error codificando el paquete: %s", err.Error()))
		panic(err)
	}

	// Enviamos el POST al servidor
	byteData := []byte(body) // castearlo a bytes antes de enviarlo
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(byteData))
	if err != nil {
		slog.Info(fmt.Sprintf("Error enviando mensajes a ip:%s puerto:%d", ip, puerto))
		panic(err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Info("Error al leer el cuerpo de la respuesta")
		panic(err)
	}

	// Verificar respuesta del servidor
	if resp.StatusCode != http.StatusOK {
		slog.Error(fmt.Sprintf("Error en la respuesta del servidor: %s", resp.Status))
		//panic("El servidor no proporciona una respuesta adecuada")
	}
	slog.Debug(fmt.Sprintf("Respuesta del servidor: %s", resp.Status))

	slog.Debug("Paquete enviado!")

	return resp, bodyBytes

}

func GenerarYEnviarPaquete2[T any](estructura *T, ip string, puerto int, ruta string) (*http.Response, []byte) {
	// URL del servidor
	url := fmt.Sprintf("http://%s:%d%s", ip, puerto, ruta)

	// Converir el paquete a formato JSON
	body, err := json.Marshal(estructura)
	if err != nil {
		slog.Error(fmt.Sprintf("Error codificando el paquete: %s", err.Error()))
		//panic(err)
		return &http.Response{StatusCode: http.StatusInternalServerError, Status: "500 Error codificando JSON"}, nil
	}

	// Enviamos el POST al servidor
	byteData := []byte(body) // castearlo a bytes antes de enviarlo
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(byteData))
	if err != nil {
		slog.Info(fmt.Sprintf("Error enviando mensajes a ip:%s puerto:%d", ip, puerto))
		//panic(err)
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Status: "503 Servicio no disponible"}, nil
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Info("Error al leer el cuerpo de la respuesta")
		//panic(err)
		return &http.Response{StatusCode: http.StatusBadGateway, Status: "502 Error leyendo respuesta"}, nil
	}

	// Verificar respuesta del servidor
	if resp.StatusCode != http.StatusOK {
		slog.Error(fmt.Sprintf("Error en la respuesta del servidor: %s", resp.Status))
		//panic("El servidor no proporciona una respuesta adecuada")
	}
	slog.Debug(fmt.Sprintf("Respuesta del servidor: %s", resp.Status))

	slog.Debug("Paquete enviado!")

	return resp, bodyBytes

}
