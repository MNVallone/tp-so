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
	"strconv"
)

// --------- VARIABLES DEL CPU --------- //
var ClientConfig *Config
var ip_memoria string = ClientConfig.IP_MEMORY
var puerto_memoria int = ClientConfig.PORT_MEMORY
var interrupcion bool

// --------- ESTRUCTURAS DEL CPU --------- //
type Config struct {
	PORT_CPU          int    `json:"port_cpu"`
	IP_CPU            string `json:"ip_cpu"`
	PORT_MEMORY       int    `json:"port_memory"`
	IP_MEMORY         string `json:"ip_memory"`
	PORT_KERNEL       int    `json:"port_kernel"`
	IP_KERNEL         string `json:"ip_kernel"`
	TLB_ENTRIES       int    `json:"tlb_entries"`
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
		TAMAÑO_PROCESO:       tamanio_proceso,
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
	interrupcion = false

	paquete := globales.PeticionCPU{}
	paquete = servidor.DecodificarPaquete(w, r, &paquete)

	// Aqui se ejecuta el proceso
	slog.Info(fmt.Sprintf("Ejecutando proceso con PID: %d", paquete.PID))

	// FASE FETCH
	for !interrupcion {
		slog.Info(fmt.Sprintf("## PID %d - FETCH - Program Counter: %d", paquete.PID, paquete.PC)) // log obligatorio
		valorPC := paquete.PC

		// Buscar instruccion a memoria con el PC del proeso
		instruccion := buscarInstruccion(paquete.PID, valorPC)

		// DECODE y EXECUTE   TODO: implementar
		decode(instruccion)
		execute(instruccion)
		// paquete.PC += 1 // Incrementar el PC para la siguiente instruccion
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func buscarInstruccion(pid int, pc int) string {
	pedidoInstruccion := globales.PeticionInstruccion{
		PC:  pc, // numero de instruccion a buscar
		PID: pid,
	}
	pidString := strconv.Itoa(pid)
	pcString := strconv.Itoa(pc)

	url := fmt.Sprintf("/cpu/buscar_instruccion/%s/%s", pidString, pcString)

	// Enviar pedido a memoria
	globales.GenerarYEnviarPaquete(&pedidoInstruccion, ip_memoria, puerto_memoria, url)

	var instruccion string

	

	return instruccion
}

func decode(instruccion string) {
	// TODO
}

func execute(instruccion string) {
	// TODO
}
