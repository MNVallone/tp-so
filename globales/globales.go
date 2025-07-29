package globales

import (
	//"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
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
	CONECTADA  bool     `json:"conectada"`
}

type SolicitudIO struct {
	NOMBRE string `json:"nombre"`
	TIEMPO int    `json:"tiempo"` // en milisegundos
	PID    int    `json:"pid"`
	PC     int    `json:"pc"`
}

type SolicitudDump struct {
	NOMBRE string `json:"nombre"`
	PID    int    `json:"pid"`
	PC     int    `json:"pc"`
}

type SolicitudProceso struct {
	ARCHIVO_PSEUDOCODIGO string `json:"archivo_pseudocodigo"`
	TAMAÑO_PROCESO       int    `json:"tamanio_proceso"`
	PID                  int    `json:"pid"`
}

type ProcesoAEjecutar struct {
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

	nivel := LogLevelFromString(log_level)

	// Handler de color que también escribe en archivo
	//colorHandler := &ColorHandler{fileWriter: logFile}
	//logger := slog.New(colorHandler)
	//slog.SetDefault(logger)

	mw := io.MultiWriter(logFile)
	log.SetOutput(mw)

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

func GenerarYEnviarPaquete[T any](estructura *T, ip string, puerto int, ruta string) (*http.Response, []byte) {
	// URL del servidor
	url := fmt.Sprintf("http://%s:%d%s", ip, puerto, ruta)

	// Converir el paquete a formato JSON
	body, err := json.Marshal(estructura)
	if err != nil {
		slog.Error(fmt.Sprintf("Error codificando el paquete: %s", err.Error()))
		//panic(err)
		return &http.Response{StatusCode: http.StatusInternalServerError, Status: "500 Error codificando JSON"}, nil
	}

	// Enviamos el POST al servidor usando un cliente con timeout fijo de 5 segundos
/* 	client := &http.Client{
		Timeout: 5 * time.Second,
	} */
	byteData := []byte(body)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(byteData))
	if err != nil {
		slog.Error(fmt.Sprintf("Error enviando mensajes a ip:%s puerto:%d", ip, puerto))
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Status: "503 Servicio no disponible"}, nil
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("Error al leer el cuerpo de la respuesta")
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

/*

func GenerarYEnviarPaquete[T any](estructura *T, ip string, puerto int, ruta string) (*http.Response, []byte) {
    // URL del servidor
    url := fmt.Sprintf("http://%s:%d%s", ip, puerto, ruta)

    // Convertir el paquete a formato JSON
    body, err := json.Marshal(estructura)
    if err != nil {
        slog.Error(fmt.Sprintf("Error codificando el paquete: %s", err.Error()))
        return &http.Response{StatusCode: http.StatusInternalServerError, Status: "500 Error codificando JSON"}, nil
    }

    // Crear la request manualmente
    req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
    if err != nil {
        slog.Error(fmt.Sprintf("Error creando la request: %s", err.Error()))
        return &http.Response{StatusCode: http.StatusInternalServerError, Status: "500 Error creando request"}, nil
    }
    req.Header.Set("Content-Type", "application/json")

    // Cliente con timeout fijo de 5 segundos
    client := &http.Client{
        Timeout: 5 * time.Second,
    }

    resp, err := client.Do(req)
    if err != nil {
        slog.Error(fmt.Sprintf("Error enviando mensajes a ip:%s puerto:%d", ip, puerto))
        return &http.Response{StatusCode: http.StatusServiceUnavailable, Status: "503 Servicio no disponible"}, nil
    }
    defer resp.Body.Close()

    bodyBytes, err := io.ReadAll(resp.Body)
    if err != nil {
        slog.Error("Error al leer el cuerpo de la respuesta")
        return &http.Response{StatusCode: http.StatusBadGateway, Status: "502 Error leyendo respuesta"}, nil
    }

    // Verificar respuesta del servidor
    if resp.StatusCode != http.StatusOK {
        slog.Error(fmt.Sprintf("Error en la respuesta del servidor: %s", resp.Status))
    }
    slog.Debug(fmt.Sprintf("Respuesta del servidor: %s", resp.Status))
    slog.Debug("Paquete enviado!")

    return resp, bodyBytes
}

*/

func DecodificarPaquete[T any](w http.ResponseWriter, r *http.Request, estructura *T) T {
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&estructura) //decodifica cualquier estructura que le pases por referencia sin importar su forma
	if err != nil {
		var zero T
		slog.Error(fmt.Sprintf("Error al decodificar mensaje: %s\n", err.Error()))
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Error al decodificar mensaje"))
		return zero // sujeto a modificaciones
	}
	return *estructura
}
