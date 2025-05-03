package utils

import (
	"encoding/json"
	"globales"
	"globales/servidor"
	"log"
	"log/slog"
	"net/http"
	"os"
	"fmt"
)

// --------- VARIABLES DEL CPU --------- //
var ClientConfig *Config

// --------- ESTRUCTURAS DEL CPU --------- //
type Config struct {
	PORT_CPU          int    `json:"port_cpu"`
	IP_CPU            string `json:"ip_cpu"`
	PORT_MEMORY       int    `json:"port_memory"`
	IP_MEMORY         string `json:"ip_memory"`
	PORT_KERNEL       int    `json:"port_kernel"`
	IP_KERNEL         string `json:"ip_kernel"`
	TLB_ENTRIES       int	 `json:"tlb_entries"`
	TLB_REPLACEMENT   string `json:"tlb_replacement"`
	CACHE_ENTRIES     int    `json:"cache_entries"`
	CACHE_REPLACEMENT string `json:"cache_replacement"`
	CACHE_DELAY       int    `json:"cache_delay"`
	LOG_LEVEL         string `json:"log_level"`
}

// --------- FUNCIONES DEL CPU --------- //
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

func IO(nombre string, tiempo int) {
	var solicitud = globales.SolicitudIO{
		NOMBRE: nombre,
		TIEMPO: tiempo,
	}
	globales.GenerarYEnviarPaquete(&solicitud, ClientConfig.IP_KERNEL, ClientConfig.PORT_KERNEL, "/cpu/solicitarIO")
}

func INIT_PROC(archivo_pseudocodigo string, tamanio_proceso int) {
	var solicitud = globales.SolicitudProceso{
		ARCHIVO_PSEUDOCODIGO: archivo_pseudocodigo,
		TAMAÑO_PROCESO: tamanio_proceso,
	}
	globales.GenerarYEnviarPaquete(&solicitud, ClientConfig.IP_KERNEL, ClientConfig.PORT_KERNEL, "/cpu/iniciarProceso")
}

func DUMP_MEMORY() { //No sabemos si pasar el PID por parametro 
	//TODO
}

func EXIT() { //No sabemos si pasar el PID por parametro 
	//TODO
}

func EjecutarProceso(w http.ResponseWriter, r *http.Request) {
	paquete := globales.PeticionCPU{}
	paquete = servidor.DecodificarPaquete(w, r, &paquete)

	// Aqui se ejecuta el proceso
	slog.Info(fmt.Sprintf("Ejecutando proceso con PID: %d", paquete.PID))


	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

