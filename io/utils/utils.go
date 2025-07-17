package utils

import (
	"encoding/json"
	"fmt"
	"globales"
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
var PIDActual int = -1

// Mutex
var mutexPeticionIO sync.Mutex
var mutexProcesamientoIO sync.Mutex

var procesandoIO chan int = make(chan int, 1)

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

type HandshakeIO struct {
	Nombre string `json:"nombre"`
	IP     string `json:"ip"`
	Puerto int    `json:"puerto"`
}

type RespuestaIO struct {
	PID                int    `json:"pid"`
	Motivo             string `json:"motivo"`
	Nombre_Dispositivo string `json:"nombre_dispositivo"`
	IP                 string `json:"ip"`
	Puerto             int    `json:"puerto"`
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

	procesandoIO <- 1

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
	slog.Debug(fmt.Sprintf("Enviado handshake al Kernel como dispositivo IO: %s", NombreDispositivo))
}

func AtenderPeticionIO(w http.ResponseWriter, r *http.Request) {
	peticion := PeticionIO{}
	peticion = globales.DecodificarPaquete(w, r, &peticion)

	// marco q estoy trabajando
	<-procesandoIO
	mutexPeticionIO.Lock()
	PIDActual = peticion.PID
	mutexPeticionIO.Unlock()

	slog.Info(fmt.Sprintf("## PID: %d - Inicio de IO - Tiempo: %d", peticion.PID, peticion.Tiempo)) // log obligatorio

	// contestar ok al kernel
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))

	// arranco la io en paralelo
	go procesarIO(peticion.PID, peticion.Tiempo)
}

func procesarIO(pid int, tiempo int) {
	// simular uso de io
	time.Sleep(time.Duration(tiempo) * time.Millisecond)

	slog.Info(fmt.Sprintf("## PID: %d - Fin de IO", pid)) // log obligatorio

	respuesta := RespuestaIO{
		PID:                pid,
		Motivo:             "Finalizo IO",
		Nombre_Dispositivo: NombreDispositivo,
		IP:                 ClientConfig.IP_IO,
		Puerto:             ClientConfig.PORT_IO,
	}

	// contesto al kernel
	ip_kernel := ClientConfig.IP_KERNEL
	puerto_kernel := ClientConfig.PORT_KERNEL
	slog.Debug(fmt.Sprintf("## PID: %d - Envio respuesta al kernel", pid))
	globales.GenerarYEnviarPaquete(&respuesta, ip_kernel, puerto_kernel, "/io/finalizado")

	mutexProcesamientoIO.Lock()
	// libero todo para procesar el siguiente
	PIDActual = -1
	mutexProcesamientoIO.Unlock()
	procesandoIO <- 1
}
