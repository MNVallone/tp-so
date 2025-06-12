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
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// --------- VARIABLES DEL CPU --------- //
var ClientConfig *Config
var desalojar bool
var ejecutandoPID int // lo agregamos para poder ejecutar exit y dump_memory
var ModificarPC bool  // si ejecutamos un GOTO o un IO, no incrementamos el PC
var PC int
var IdCpu string
var dejarDeEjecutar bool

var TamanioPagina int
var CantidadEntradas int
var CantidadNiveles int

var algoritmoTLB string // FIFO o LRU

var TLB []globales.EntradaTLB

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

	algoritmoTLB = config.TLB_REPLACEMENT

	return config
}

func EjecutarProceso(w http.ResponseWriter, r *http.Request) {
	desalojar = false
	dejarDeEjecutar = false

	paquete := globales.PeticionCPU{}
	paquete = servidor.DecodificarPaquete(w, r, &paquete)

	// Aqui se ejecuta el proceso
	slog.Info(fmt.Sprintf("Ejecutando proceso con PID: %d", paquete.PID))
	ejecutandoPID = paquete.PID

	PC = paquete.PC

	for !desalojar && !dejarDeEjecutar {
		ModificarPC = true // por defecto incrementamos el PC

		slog.Debug(fmt.Sprintf("## PID %d - FETCH - Program Counter: %d", paquete.PID, PC)) // log obligatorio
		// FASE FETCH
		instruccion := buscarInstruccion(paquete.PID, PC) // Buscar instruccion a memoria con el PC del proeso

		// DECODE y EXECUTE
		DecodeAndExecute(instruccion)
		if ModificarPC { // el if es por si ejecuta GOTO
			PC++
		}
	}

	// CHECK_INTERRUPT
	if desalojar && !dejarDeEjecutar {
		procesoInterrumpido := globales.Interrupcion{
			PID: ejecutandoPID,
			PC:  PC,
		}
		globales.GenerarYEnviarPaquete(&procesoInterrumpido, ClientConfig.IP_KERNEL, ClientConfig.PORT_KERNEL, "/cpu/interrupt")
	}

	handshakeCPU := globales.HandshakeCPU{
		ID_CPU:   IdCpu,
		PORT_CPU: ClientConfig.PORT_CPU, // 8004
		IP_CPU:   ClientConfig.IP_CPU,
	}

	go globales.GenerarYEnviarPaquete(&handshakeCPU, ClientConfig.IP_KERNEL, ClientConfig.PORT_KERNEL, "/cpu/handshake")

	EliminarEntradasTLB()

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

func traduccionDireccionLogica(nroPagina int, direccionLogica int) int {
	if EstaEnTLB(nroPagina) { // TLB Hit
		nroMarcoInt := obtenerMarcoTLB(nroPagina)
		offset := direccionLogica % TamanioPagina
		slog.Info(fmt.Sprintf("PID: %d - TLB HIT - Pagina: %d", ejecutandoPID, nroPagina))
		// Actualizar tiempo de referencia de la entrada TLB
		for i := range TLB {
			if TLB[i].NUMERO_PAG == nroPagina {
				TLB[i].TIEMPO_DESDE_REFERENCIA = time.Now() // Actualizar el tiempo de uso de la entrada TLB
			}
		}
		return nroMarcoInt*TamanioPagina + offset // direccion fisica

	} else { // TLB Miss
		entrada_nivel_X, desplazamiento := MMU(direccionLogica)
		marcoStruct := globales.ObtenerMarco{
			PID:              ejecutandoPID,
			Entradas_Nivel_X: entrada_nivel_X,
		}

		_, nroMarco := globales.GenerarYEnviarPaquete(&marcoStruct, ClientConfig.IP_MEMORY, ClientConfig.PORT_MEMORY, "/cpu/escribir_direccion")
		marco, err := io.ReadAll(bytes.NewReader(nroMarco))

		if err != nil {
			slog.Error(fmt.Sprintf("Error al leer el cuerpo de la respuesta: %v", err))
		}
		nroMarcoInt, _ := strconv.Atoi(string(marco))

		saveTLB(nroPagina, nroMarcoInt)

		direccionFisica := nroMarcoInt*TamanioPagina + desplazamiento // direccion fisica
		slog.Debug(fmt.Sprintf("PID: %d - TLB MISS - Pagina: %d", ejecutandoPID, nroPagina))

		return direccionFisica
	}
}

func EstaEnTLB(numeroDePagina int) bool {
	for _, entrada := range TLB {
		if entrada.NUMERO_PAG == numeroDePagina {
			// hay que actualizar el tiempo de referencia??
			return true // TLB Hit
		}
	}
	return false // TLB Miss
}

func saveTLB(nroPagina int, nroMarco int) {
	nuevaEntradaTLB := globales.EntradaTLB{
		NUMERO_PAG:              nroPagina,
		NUMERO_MARCO:            nroMarco,
		TIEMPO_DESDE_REFERENCIA: time.Now(), //Agregar en READ tambien
	}

	if len(TLB) < ClientConfig.TLB_ENTRIES {
		TLB = append(TLB, nuevaEntradaTLB) // Agregar nueva entrada si hay espacio

		return
	}
	// Reemplazo de TLB
	if algoritmoTLB == "FIFO" {
		TLB = append(TLB[1:], nuevaEntradaTLB) // Reemplazamos siempre la primera entrada
	} else { // LRU: Ver si acomodamos la TLB antes de reemplazar y siempre sacar el primerop
		indiceMenosUsado := 0

		// el que hace mas tiempo que no se referencia, es el que mas TIEMPO_DESDE_REFERENCIA tiene
		for i, entrada := range TLB {
			if entrada.TIEMPO_DESDE_REFERENCIA.Before(TLB[indiceMenosUsado].TIEMPO_DESDE_REFERENCIA) {
				indiceMenosUsado = i
			}
		}
		TLB[indiceMenosUsado] = nuevaEntradaTLB // Replazamos el indice de la posicionque hace mas tiempo no se referencia
	}
}

func obtenerMarcoTLB(nroPagina int) int {
	for _, entrada := range TLB {
		if entrada.NUMERO_PAG == nroPagina {
			return entrada.NUMERO_MARCO
		}
	}
	slog.Error(fmt.Sprintf("No se encontró el marco para la página %d en la TLB", nroPagina))
	return -1 // Si no se encuentra, retornar un valor inválido
}

func InterrumpirPorDesalojo(w http.ResponseWriter, r *http.Request) {
	var peticion globales.Interrupcion
	peticion = servidor.DecodificarPaquete(w, r, &peticion)

	if peticion.PID != ejecutandoPID {
		slog.Error(fmt.Sprintf("La interrupción recibida no corresponde al PID %d, sino al PID %d", ejecutandoPID, peticion.PID))
		desalojar = false
	} else {
		desalojar = true
		slog.Info(fmt.Sprintf("Interrupción recibida para PID %d, PC actualizado a %d", peticion.PID, PC))
	}

	slog.Info("## Llega interrupcion al puerto Interrupt")

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func WRITE(direccionLogica int, datos string) {
	var direccionFisica int
	nroPagina := direccionLogica / TamanioPagina
	direccionFisica = traduccionDireccionLogica(nroPagina, direccionLogica)

	peticion := globales.EscribirMemoria{
		DIRECCION: direccionFisica,
		DATOS:     datos,
	}

	resp, _ := globales.GenerarYEnviarPaquete(&peticion, ClientConfig.IP_MEMORY, ClientConfig.PORT_MEMORY, "/cpu/escribir_direccion")
	if resp.StatusCode != http.StatusOK {
		slog.Error(fmt.Sprintf("Error al escribir en memoria: %s", resp.Status))
		return
	} else {
		slog.Info(fmt.Sprintf("PID: %d - Acción: ESCRIBIR - Dirección Física: %d - Valor Escrito: %s", ejecutandoPID, direccionFisica, datos))
	}

}

func READ(direccionLogica int, tamanio int) {
	var direccionFisica int
	nroPagina := direccionLogica / TamanioPagina
	direccionFisica = traduccionDireccionLogica(nroPagina, direccionLogica)

	peticion := globales.LeerMemoria{
		DIRECCION: direccionFisica,
		TAMANIO:   tamanio,
	}

	resp, body := globales.GenerarYEnviarPaquete(&peticion, ClientConfig.IP_MEMORY, ClientConfig.PORT_MEMORY, "/cpu/leer_direccion")
	if resp.StatusCode != http.StatusOK {
		slog.Error(fmt.Sprintf("Error al escribir en memoria: %s", resp.Status))
		return
	} else {
		contenido, err := io.ReadAll(bytes.NewReader(body))
		if err == nil {
			slog.Info(fmt.Sprintf("PID: %d - Acción: LEER - Dirección Física: %d - Valor Leido: %s", ejecutandoPID, direccionFisica, string(contenido)))
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

// Direcciones logicas
func MMU(direccionLogica int) (entrada_nivel_X []int, desplazamiento int) {
	nroPagina := direccionLogica / TamanioPagina
	desplazamiento = direccionLogica % TamanioPagina
	entrada_nivel_X = make([]int, CantidadNiveles)
	for x := 1; x <= CantidadNiveles; x++ {
		divisor := int(math.Pow(float64(CantidadEntradas), float64(CantidadNiveles-x)))
		entrada_nivel_X[x-1] = (nroPagina / divisor) % CantidadEntradas
	}
	return
}

func EliminarEntradasTLB() {
	TLB = []globales.EntradaTLB{} // Limpiar TLB
	slog.Info("Se han eliminado las entradas de la TLB del proceso")
}
