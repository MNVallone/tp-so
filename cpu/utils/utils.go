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

	paquete := globales.PeticionCPU{}
	paquete = servidor.DecodificarPaquete(w, r, &paquete)

	// Aqui se ejecuta el proceso
	slog.Info(fmt.Sprintf("Ejecutando proceso con PID: %d", paquete.PID))
	ejecutandoPID = paquete.PID

	PC = paquete.PC
	// FASE FETCH
	for !interrupcion {
		ModificarPC = true // por defecto incrementamos el PC

		slog.Info(fmt.Sprintf("## PID %d - FETCH - Program Counter: %d", paquete.PID, PC)) // log obligatorio

		// Buscar instruccion a memoria con el PC del proeso
		instruccion := buscarInstruccion(paquete.PID, PC)

		// DECODE y EXECUTE
		DecodeAndExecute(instruccion)
		if ModificarPC {
			PC++ // Incrementar el PC para la siguiente instruccion
		}

	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func buscarInstruccion(pid int, pc int) string {
	pedidoInstruccion := globales.PeticionInstruccion{
		PC:  pc,
		PID: pid,
	}
	// pidString := strconv.Itoa(pid)
	// pcString := strconv.Itoa(pc)

	// url := fmt.Sprintf("/cpu/buscar_instruccion/%s/%s", pidString, pcString)

	// Enviar pedido a memoria
	var resp *http.Response = globales.GenerarYEnviarPaquete(&pedidoInstruccion, ClientConfig.IP_MEMORY, ClientConfig.PORT_MEMORY, "/cpu/buscar_instruccion")

	// Recibir respuesta de memoria
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error(fmt.Sprintf("Error al leer el cuerpo de la respuesta: %v", err))
		panic("Error al leer el cuerpo de la respuesta")
	}

	// Convertir los bytes del cuerpo a un string.
	bodyString := string(bodyBytes)
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
	case "IO":
		nombre := sliceInstruccion[1]
		tiempo, err := strconv.Atoi(sliceInstruccion[2])
		if err == nil {
			IO(nombre, tiempo)
		}
	case "INIT_PROC":
		archivoDeInstrucc := sliceInstruccion[1]
		tamanio, err := strconv.Atoi(sliceInstruccion[2])
		if err == nil {
			INIT_PROC(archivoDeInstrucc, tamanio)
		}

	case "DUMP_MEMORY":
		DUMP_MEMORY()

	case "EXIT":
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

	resp := globales.GenerarYEnviarPaquete(&peticion, ClientConfig.IP_MEMORY, ClientConfig.PORT_MEMORY, "/cpu/escribir_direccion")
	if resp.StatusCode != http.StatusOK {
		slog.Error(fmt.Sprintf("Error al escribir en memoria: %s", resp.Status))
		return
	} else {
		slog.Info(fmt.Sprintf("PID: %d - Acción: ESCRIBIR - Dirección Física: %d - Valor Escrito: %s", ejecutandoPID, direccion, datos))
	}

}

func READ(direccion int, tamanio int) {
	//TODO
	//Traducir direccion lógica a física con MMU en siguientes implementaciones
	peticion := globales.LeerMemoria{
		DIRECCION: direccion,
		TAMANIO:   tamanio,
	}

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
