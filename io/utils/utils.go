package utils

import (
	"encoding/json"
	"fmt"
	"globales"
	"globales/servidor"
	"log"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"
)

// --------- VARIABLES DE IO --------- //
var ClientConfig *Config
var NombreDispositivo string
var ProcesandoIO bool = false
var PIDActual int = 0

// Mutex
var mutexPeticionIO sync.Mutex
var mutexProcesamientoIO sync.Mutex

// --------- ESTRUCTURAS DE IO --------- //
type Config struct {
	PORT_IO     int    `json:"port_io"`
	IP_IO       string `json:"ip_io"`
	IP_KERNEL   string `json:"ip_kernel"`
	PORT_KERNEL int    `json:"port_kernel"`
	LOG_LEVEL   string `json:"log_level"`
}

type PeticionIO struct {
	PID    int `json:"pid"`
	Tiempo int `json:"tiempo"`
}

type RespuestaIO struct {
	PID    int    `json:"pid"`
	Estado string `json:"estado"`
}

type HandshakeIO struct {
	Nombre string `json:"nombre"`
	IP     string `json:"ip"`
	Puerto int    `json:"puerto"`
}

type RespuestaIO struct {
	PID                int    `json:"pid"`
	Nombre_Dispositivo string `json:"nombre_dispositivo"`
}

// --------- FUNCIONES DE IO --------- //
func IniciarConfiguracion(filePath string) *Config {
	var config *Config
	configFile, err := os.Open(filePath)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer configFile.Close()

	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&config)

	return config
}

// handshake con el kernel
func RealizarHandshake(ip_kernel string, puerto_kernel int) {
	handshake := HandshakeIO{
		Nombre: NombreDispositivo,
		IP:     ClientConfig.IP_IO,
		Puerto: ClientConfig.PORT_IO,
	}

	// armo el mensaje con el nombre del disp IO
	globales.GenerarYEnviarPaquete(&handshake, ip_kernel, puerto_kernel, "/io/handshake")
	slog.Info(fmt.Sprintf("Enviado handshake al Kernel como dispositivo IO: %s", NombreDispositivo))
}

func AtenderPeticionIO(w http.ResponseWriter, r *http.Request) {
	peticion := PeticionIO{}
	peticion = servidor.DecodificarPaquete(w, r, &peticion)

	mutexPeticionIO.Lock()
	if ProcesandoIO {
		// si ya estoy procesando contesto que estoy ocupado
		slog.Info(fmt.Sprintf("Dispositivo %s ocupado, no puede procesar PID %d", NombreDispositivo, peticion.PID))
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("dispositivo ocupado"))
		mutexPeticionIO.Unlock()
		return
	}

	// marco q estoy trabajando
	ProcesandoIO = true
	PIDActual = peticion.PID
	mutexPeticionIO.Unlock()

	slog.Info(fmt.Sprintf("## PID: %d - Inicio de IO - Tiempo: %d", peticion.PID, peticion.Tiempo))

	// contestar ok al kernel
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))

	// arranco la io en paralelo
	//go procesarIO(peticion.PID, peticion.Tiempo)
	go procesarIOVol2(peticion.PID, peticion.Tiempo)
}

func procesarIO(pid int, tiempo int) {
	// simular uso de io
	time.Sleep(time.Duration(tiempo) * time.Millisecond)

	slog.Info(fmt.Sprintf("## PID: %d - Fin de IO", pid))

	respuesta := RespuestaIO{
		PID:    pid,
		Estado: "finalizado",
	}

	// contesto al kernel
	ip_kernel := ClientConfig.IP_KERNEL
	puerto_kernel := ClientConfig.PORT_KERNEL
	globales.GenerarYEnviarPaquete(&respuesta, ip_kernel, puerto_kernel, "/io/finalizado")

	mutexProcesamientoIO.Lock()
	// libero todo para procesar el siguiente
	ProcesandoIO = false
	PIDActual = 0
	mutexProcesamientoIO.Unlock()
}

func procesarIOVol2(pid int, tiempo int) {
	// simular uso de io
	time.Sleep(time.Duration(tiempo) * time.Millisecond)

	slog.Info(fmt.Sprintf("## PID: %d - Fin de IO", pid))

	respuesta := RespuestaIO{
		PID:                pid,
		Nombre_Dispositivo: NombreDispositivo,
	}

	// contesto al kernel
	ip_kernel := ClientConfig.IP_KERNEL
	puerto_kernel := ClientConfig.PORT_KERNEL
	slog.Info(fmt.Sprintf("## PID: %d - Envio respuesta al kernel", pid))
	globales.GenerarYEnviarPaquete(&respuesta, ip_kernel, puerto_kernel, "/io/finalizado")

	mutexProcesamientoIO.Lock()
	// libero todo para procesar el siguiente
	ProcesandoIO = false
	PIDActual = 0
	mutexProcesamientoIO.Unlock()
}
