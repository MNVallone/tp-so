package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"globales"
	"globales/servidor"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// --------- VARIABLES DEL CPU --------- //
var ClientConfig *Config
var interrupcion bool
var ejecutandoPID int // lo agregamos para poder ejecutar exit y dump_memory
var ModificarPC bool  // si ejecutamos un GOTO o un IO, no incrementamos el PC
var PC int
var IdCpu string
var dejarDeEjecutar bool

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

func EjecutarProceso(w http.ResponseWriter, r *http.Request) {
	interrupcion = false
	dejarDeEjecutar = false

	paquete := globales.PeticionCPU{}
	paquete = servidor.DecodificarPaquete(w, r, &paquete)

	// Aqui se ejecuta el proceso
	slog.Info(fmt.Sprintf("Ejecutando proceso con PID: %d", paquete.PID))
	ejecutandoPID = paquete.PID

	PC = paquete.PC

	// FASE FETCH
	for !interrupcion && !dejarDeEjecutar {
		ModificarPC = true // por defecto incrementamos el PC

		slog.Debug(fmt.Sprintf("## PID %d - FETCH - Program Counter: %d", paquete.PID, PC)) // log obligatorio

		instruccion := buscarInstruccion(paquete.PID, PC) // Buscar instruccion a memoria con el PC del proeso

		// DECODE y EXECUTE
		DecodeAndExecute(instruccion)
		if ModificarPC { // el if es por si ejecuta GOTO
			PC++
		}

	}

	if interrupcion {
		procesoInterrumpido := globales.Interrupcion{
			PID:    ejecutandoPID,
			PC:     PC,
			MOTIVO: "",
		}
		globales.GenerarYEnviarPaquete(&procesoInterrumpido, ClientConfig.IP_KERNEL, ClientConfig.PORT_KERNEL, "/cpu/interrupt")
	}

	handshakeCPU := globales.HandshakeCPU{
		ID_CPU:   IdCpu,
		PORT_CPU: ClientConfig.PORT_CPU, // 8004
		IP_CPU:   ClientConfig.IP_CPU,
	}
	globales.GenerarYEnviarPaquete(&handshakeCPU, ClientConfig.IP_KERNEL, ClientConfig.PORT_KERNEL, "/cpu/handshake")

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func buscarInstruccion(pid int, pc int) string {
	pedidoInstruccion := globales.PeticionInstruccion{
		PC:  pc,
		PID: pid,
	}

	// Enviar pedido a memoria
	_, respBody := globales.GenerarYEnviarPaquete(&pedidoInstruccion, ClientConfig.IP_MEMORY, ClientConfig.PORT_MEMORY, "/cpu/buscar_instruccion")

	// Convertir los bytes del cuerpo a un string.
	bodyString := string(respBody)
	var instruccion string

	json.Unmarshal([]byte(bodyString), &instruccion)

	return instruccion
}

func DecodeAndExecute(instruccion string) {
	sliceInstruccion := strings.Split(instruccion, " ")
	switch sliceInstruccion[0] {
	case "NOOP":
	case "WRITE":
		datos := sliceInstruccion[2]
		direccion, err := strconv.Atoi(sliceInstruccion[1])
		if err == nil { // sacar si hay que sumarle 1 al PC
			WRITE(direccion, datos)
		}

	case "READ":
		direccion, err1 := strconv.Atoi(sliceInstruccion[1])
		tamanio, err2 := strconv.Atoi(sliceInstruccion[2])
		if err1 == nil && err2 == nil { // sacar si hay que sumarle 1 al PC
			READ(direccion, tamanio)
		}

	case "GOTO":
		ModificarPC = false
		nuevoPC, err := strconv.Atoi(sliceInstruccion[1])
		if err == nil { // sacar si hay que sumarle 1 al PC
			PC = nuevoPC
		}
	case "IO": // syscall
		nombre := sliceInstruccion[1]
		tiempo, err := strconv.Atoi(sliceInstruccion[2])
		if err == nil {
			IO(nombre, tiempo)
		}
	case "INIT_PROC": // syscall
		archivoDeInstrucc := sliceInstruccion[1]
		tamanio, err := strconv.Atoi(sliceInstruccion[2])
		if err == nil {
			INIT_PROC(archivoDeInstrucc, tamanio)
		}

	case "DUMP_MEMORY": // syscall
		DUMP_MEMORY()

	case "EXIT": // syscall
		EXIT()
	}
}

func WRITE(direccion int, datos string) {
	//TODO

	//Traducir direccion logica a fisica

	peticion := globales.EscribirMemoria{
		DIRECCION: direccion,
		DATOS:     datos,
	}

	resp, _ := globales.GenerarYEnviarPaquete(&peticion, ClientConfig.IP_MEMORY, ClientConfig.PORT_MEMORY, "/cpu/escribir_direccion")
	if resp.StatusCode != http.StatusOK {
		slog.Error(fmt.Sprintf("Error al escribir en memoria: %s", resp.Status))
		return
	} else {
		slog.Info(fmt.Sprintf("PID: %d - Acción: ESCRIBIR - Dirección Física: %d - Valor Escrito: %s", ejecutandoPID, direccion, datos))
	}

}

func CHECK_INTERRUPT(w http.ResponseWriter, r *http.Request) {
	interrupcion = true

	slog.Info("Llega interrupcion al puerto Interrupt.")

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func READ(direccion int, tamanio int) {
	//TODO
	//Traducir direccion lógica a física con MMU en siguientes implementaciones
	peticion := globales.LeerMemoria{
		DIRECCION: direccion,
		TAMANIO:   tamanio,
	}

	//TODO: usar la nueva version de GenerarYEnviarPaquete que devuelve el body
	url := fmt.Sprintf("http://%s:%d%s", ClientConfig.IP_MEMORY, ClientConfig.PORT_MEMORY, "/cpu/leer_direccion")

	// Converir el paquete a formato JSON
	body, err := json.Marshal(peticion)
	if err != nil {
		slog.Error(fmt.Sprintf("Error codificando el paquete: %s", err.Error()))
		panic(err)
	}

	// Enviamos el POST al servidor
	byteData := []byte(body) // castearlo a bytes antes de enviarlo
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(byteData))
	if err != nil {
		slog.Info(fmt.Sprintf("Error enviando mensajes a ip:%s puerto:%d", ClientConfig.IP_MEMORY, ClientConfig.PORT_MEMORY))
		panic(err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Error(fmt.Sprintf("Error al escribir en memoria: %s", resp.Status))
		return
	} else {
		contenido, err := io.ReadAll(resp.Body)
		if err == nil {
			slog.Info(fmt.Sprintf("PID: %d - Acción: LEER - Dirección Física: %d - Valor Leido: %s", ejecutandoPID, direccion, string(contenido)))
		} else {
			fmt.Print("error leyendo body")
		}
	}

}

func IO(nombre string, tiempo int) {
	var solicitud = globales.SolicitudIO{
		NOMBRE: nombre,
		TIEMPO: tiempo,
		PID:    ejecutandoPID,
		PC:     PC + 1,
	}
	globales.GenerarYEnviarPaquete(&solicitud, ClientConfig.IP_KERNEL, ClientConfig.PORT_KERNEL, "/cpu/solicitarIO")
	dejarDeEjecutar = true
}

func INIT_PROC(archivo_pseudocodigo string, tamanio_proceso int) {
	var solicitud = globales.SolicitudProceso{
		ARCHIVO_PSEUDOCODIGO: archivo_pseudocodigo,
		TAMAÑO_PROCESO:       tamanio_proceso,
	}
	globales.GenerarYEnviarPaquete(&solicitud, ClientConfig.IP_KERNEL, ClientConfig.PORT_KERNEL, "/cpu/iniciarProceso")
}

func DUMP_MEMORY() {
	var solicitud = globales.SolicitudDump{
		PID: ejecutandoPID,
		PC:  PC + 1,
	}
	globales.GenerarYEnviarPaquete(&solicitud, ClientConfig.IP_KERNEL, ClientConfig.PORT_KERNEL, "/cpu/dumpearMemoria")
	dejarDeEjecutar = true
}

func EXIT() {
	var pid = globales.PID{
		NUMERO_PID: ejecutandoPID,
	}

	globales.GenerarYEnviarPaquete(&pid, ClientConfig.IP_KERNEL, ClientConfig.PORT_KERNEL, "/cpu/terminarProceso")
	slog.Debug(fmt.Sprintf("PID: %d - Acción: EXIT", ejecutandoPID))
	dejarDeEjecutar = true
}
