package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"globales"
	"io"
	"log"
	"log/slog"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type EntradaTLB struct {
	NUMERO_PAG              int       `json:"numero_pagina"`        // Número de página
	NUMERO_MARCO            int       `json:"numero_marco"`         // Número de marco de página
	TIEMPO_DESDE_REFERENCIA time.Time `json:"tiempo_de_referencia"` // Dirección física del marco de página
}

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

var cacheHabilitada bool = false
var tlbHabilitada bool = false

var algoritmoTLB string   // FIFO o LRU
var algoritmoCache string // CLOCK o CLOCK-M

var TLB []EntradaTLB

var MemoriaCache []EntradaCache // Cache de memoria

var punteroMemoriaCache int // Puntero para la cache, para saber donde escribir la proxima entrada

var mutexEjecucion sync.Mutex

type EntradaCache struct {
	nroPagina      int
	Datos          []byte
	nroMarco       int // para facilitar la traduccion de direccion logica a fisica
	bitDeUso       bool
	bitModificado  bool
	entradaOcupada bool
}

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

// --------- INICIALIZACION DEL MODULO --------- //
func IniciarConfiguracion(filePath string) *Config {
	var config *Config
	configFile, err := os.Open(filePath)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer configFile.Close()

	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&config)

	slog.Debug(fmt.Sprintf("Configuración cargada: %+v", *config))

	algoritmoTLB = config.TLB_REPLACEMENT
	algoritmoCache = config.CACHE_REPLACEMENT

	if config.CACHE_ENTRIES > 0 {
		cacheHabilitada = true
		MemoriaCache = make([]EntradaCache, config.CACHE_ENTRIES)
		for i := range MemoriaCache {
			MemoriaCache[i] = EntradaCache{
				nroPagina:      -1, // Inicializamos con -1 para indicar que no hay pagina cargada
				Datos:          nil,
				bitDeUso:       false,
				bitModificado:  false,
				entradaOcupada: false,
			}
		}

		punteroMemoriaCache = 0 // Inicializamos el puntero de la cache
	}

	if config.TLB_ENTRIES > 0 {
		tlbHabilitada = true
	}

	slog.Debug(fmt.Sprintf("%v", tlbHabilitada))

	return config
}

// --------- CICLO DE INSTRUCCIÓN --------- //
func EjecutarProceso(w http.ResponseWriter, r *http.Request) {

	desalojar = false
	dejarDeEjecutar = false

	paquete := globales.ProcesoAEjecutar{}
	paquete = globales.DecodificarPaquete(w, r, &paquete)

	// Aqui se ejecuta el proceso
	// slog.Info(fmt.Sprintf("Ejecutando proceso con PID: %d", paquete.PID))
	ejecutandoPID = paquete.PID

	PC = paquete.PC

	slog.Debug(fmt.Sprintf("CPU %s ejecutando PID %d en PC %d", IdCpu, paquete.PID, paquete.PC))

	for !desalojar && !dejarDeEjecutar {
		// time.Sleep(100 * time.Millisecond)
		mutexEjecucion.Lock()
		ModificarPC = true // por defecto incrementamos el PC

		slog.Debug(fmt.Sprintf("## PID %d - FETCH - Program Counter: %d", paquete.PID, PC)) // log obligatorio
		// FASE FETCH
		instruccion := buscarInstruccion(paquete.PID, PC) // Buscar instruccion a memoria con el PC del proeso

		// DECODE y EXECUTE
		DecodeAndExecute(instruccion)
		if ModificarPC { // el if es por si ejecuta GOTO
			PC++
		}
		mutexEjecucion.Unlock()
	}

	// CHECK_INTERRUPT
	if desalojar && !dejarDeEjecutar {
		procesoInterrumpido := globales.Interrupcion{
			PID: ejecutandoPID,
			PC:  PC,
		}
		slog.Debug("ENVIANDO PROCESO INTERRUMPIDO")
		go globales.GenerarYEnviarPaquete(&procesoInterrumpido, ClientConfig.IP_KERNEL, ClientConfig.PORT_KERNEL, "/cpu/interrupt")
	}

	handshakeCPU := globales.HandshakeCPU{
		ID_CPU:   IdCpu,
		PORT_CPU: ClientConfig.PORT_CPU,
		IP_CPU:   ClientConfig.IP_CPU,
		//DISPONIBLE: nil,
	}

	slog.Debug(fmt.Sprintf("Desalojar proceso: %t, dejar de ejecutar: %t", desalojar, dejarDeEjecutar))

	slog.Debug(fmt.Sprintf("Entradas TLB: %v", TLB))
	EliminarEntradasTLB()
	limpiarCache()

	slog.Debug("RECONECTANDOME CON KERNEL")
	go globales.GenerarYEnviarPaquete(&handshakeCPU, ClientConfig.IP_KERNEL, ClientConfig.PORT_KERNEL, "/cpu/handshake")
	slog.Debug("RECONECTADO CON KERNEL")

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

	nombreInstruccion := sliceInstruccion[0]
	parametros := sliceInstruccion[1:]

	slog.Info(fmt.Sprintf("## PID: %d - Ejecutando: %s - %s", ejecutandoPID, nombreInstruccion, parametros)) // log obligatorio

	switch nombreInstruccion {
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

// --------- INTERRUMPIR UN PROCESO POR DESALOJO --------- //
func InterrumpirPorDesalojo(w http.ResponseWriter, r *http.Request) {
	mutexEjecucion.Lock()
	var peticion globales.Interrupcion
	peticion = globales.DecodificarPaquete(w, r, &peticion)

	if peticion.PID != ejecutandoPID {
		slog.Error(fmt.Sprintf("La interrupción recibida no corresponde al PID %d, sino al PID %d", ejecutandoPID, peticion.PID))
		desalojar = false
	} else {
		desalojar = true
		slog.Debug(fmt.Sprintf("Interrupción recibida para PID %d, PC actualizado a %d", peticion.PID, PC))
	}
	mutexEjecucion.Unlock()

	slog.Info("## Llega interrupcion al puerto Interrupt") // log obligatorio

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// --------- INSTRUCCIONES --------- //
func WRITE(direccionLogica int, datos string) {

	if cacheHabilitada {
		nroPagina := direccionLogica / TamanioPagina
		offset := direccionLogica % TamanioPagina
		indiceEntradaCache := buscarEntradaCache(nroPagina, direccionLogica)
		contenidoPagina := MemoriaCache[indiceEntradaCache].Datos

		//contenido := contenidoPagina[offset : offset+tamanio] // Obtenemos el contenido de la pagina desde el offset hasta el tamanio solicitado
		slog.Debug(fmt.Sprintf("PID: %d - WRITE - Pagina: %d, Offset: %d, Datos: %s , IndiceCache: %d", ejecutandoPID, nroPagina, offset, datos, indiceEntradaCache))
		direccionFisica := MemoriaCache[indiceEntradaCache].nroMarco*TamanioPagina + offset // direccion fisica
		j := 0

		slog.Debug(fmt.Sprintf("LONGITUD CONTENIDO DE LA PAGINA: %d", len(contenidoPagina)))

		for i := offset; i < TamanioPagina && j < len(datos) && offset+i < len(contenidoPagina); i++ {
			slog.Debug(fmt.Sprintf("Escribiendo en la pagina: %d, i: %d ,j: %d , Datos: %s", nroPagina, i, j, string(datos[j])))
			contenidoPagina[offset+i] = datos[j]
			j++
		}

		slog.Debug(fmt.Sprintf("Contenido de la pagina despues de escribir: %s", string(contenidoPagina)))

		MemoriaCache[indiceEntradaCache].Datos = contenidoPagina // Actualizamos los datos de la pagina en la cache

		MemoriaCache[indiceEntradaCache].bitModificado = true // Marcamos la pagina como modificada

		slog.Info(fmt.Sprintf("PID: %d - Acción: ESCRIBIR - Dirección Física: %d - Valor: %s", ejecutandoPID, direccionFisica, string(datos))) // log obligatorio

	} else {
		var direccionFisica int
		nroPagina := direccionLogica / TamanioPagina
		offset := direccionLogica % TamanioPagina
		nroMarco := traduccionDireccionLogica(nroPagina, direccionLogica)

		direccionFisica = nroMarco*TamanioPagina + offset

		peticion := globales.EscribirMemoria{
			DIRECCION: direccionFisica,
			PID:       ejecutandoPID,
			DATOS:     datos,
		}

		resp, _ := globales.GenerarYEnviarPaquete(&peticion, ClientConfig.IP_MEMORY, ClientConfig.PORT_MEMORY, "/cpu/escribir_direccion")
		if resp.StatusCode != http.StatusOK {
			slog.Error(fmt.Sprintf("Error al escribir en memoria: %s", resp.Status))
			return
		} else {
			slog.Info(fmt.Sprintf("PID: %d - Acción: ESCRIBIR - Dirección Física: %d - Valor: %s", ejecutandoPID, direccionFisica, datos)) // log obligatorio
		}
	}
}

func READ(direccionLogica int, tamanio int) {

	if cacheHabilitada {
		nroPagina := direccionLogica / TamanioPagina
		offset := direccionLogica % TamanioPagina
		indiceEntradaCache := buscarEntradaCache(nroPagina, direccionLogica)
		contenidoPagina := MemoriaCache[indiceEntradaCache].Datos
		slog.Debug(fmt.Sprintf("PID: %d - LEER - Pagina: %d, Offset: %d , IndiceCache: %d", ejecutandoPID, nroPagina, offset, indiceEntradaCache))

		contenido := contenidoPagina[offset : offset+tamanio] // Obtenemos el contenido de la pagina desde el offset hasta el tamanio solicitado

		direccionFisica := MemoriaCache[indiceEntradaCache].nroMarco*TamanioPagina + offset                                                    // direccion fisica
		slog.Info(fmt.Sprintf("PID: %d - Acción: LEER - Dirección Física: %d - Valor: %s", ejecutandoPID, direccionFisica, string(contenido))) // log obligatorio

	} else {

		var direccionFisica int
		nroPagina := direccionLogica / TamanioPagina
		offset := direccionLogica % TamanioPagina
		nroMarco := traduccionDireccionLogica(nroPagina, direccionLogica)

		direccionFisica = nroMarco*TamanioPagina + offset

		peticion := globales.LeerMemoria{
			DIRECCION: direccionFisica,
			PID:       ejecutandoPID,
			TAMANIO:   tamanio,
		}

		resp, body := globales.GenerarYEnviarPaquete(&peticion, ClientConfig.IP_MEMORY, ClientConfig.PORT_MEMORY, "/cpu/leer_direccion")
		if resp.StatusCode != http.StatusOK {
			slog.Error(fmt.Sprintf("Error al escribir en memoria: %s", resp.Status))
			return
		} else {
			contenido, err := io.ReadAll(bytes.NewReader(body))
			if err == nil {
				slog.Info(fmt.Sprintf("PID: %d - Acción: LEER - Dirección Física: %d - Valor: %s", ejecutandoPID, direccionFisica, string(contenido))) // log obligatorio
			} else {
				fmt.Print("error leyendo body")
			}
		}
	}
}

// --------- SYSCALLS --------- //
func IO(nombre string, tiempo int) {
	var solicitud = globales.SolicitudIO{
		NOMBRE: nombre,
		TIEMPO: tiempo,
		PID:    ejecutandoPID,
		PC:     PC + 1,
	}
	go globales.GenerarYEnviarPaquete(&solicitud, ClientConfig.IP_KERNEL, ClientConfig.PORT_KERNEL, "/cpu/solicitarIO")

	dejarDeEjecutar = true
}

func INIT_PROC(archivo_pseudocodigo string, tamanio_proceso int) {
	var solicitud = globales.SolicitudProceso{
		ARCHIVO_PSEUDOCODIGO: archivo_pseudocodigo,
		TAMAÑO_PROCESO:       tamanio_proceso,
		PID:                  ejecutandoPID,
	}
	go globales.GenerarYEnviarPaquete(&solicitud, ClientConfig.IP_KERNEL, ClientConfig.PORT_KERNEL, "/cpu/iniciarProceso")
}

func DUMP_MEMORY() {
	var solicitud = globales.SolicitudDump{
		PID: ejecutandoPID,
		PC:  PC + 1,
	}
	go globales.GenerarYEnviarPaquete(&solicitud, ClientConfig.IP_KERNEL, ClientConfig.PORT_KERNEL, "/cpu/dumpearMemoria")
	dejarDeEjecutar = true
}

func EXIT() {
	var pid = globales.PID{
		NUMERO_PID: ejecutandoPID,
	}

	go globales.GenerarYEnviarPaquete(&pid, ClientConfig.IP_KERNEL, ClientConfig.PORT_KERNEL, "/cpu/terminarProceso")
	slog.Debug(fmt.Sprintf("PID: %d - Acción: EXIT", ejecutandoPID))
	dejarDeEjecutar = true
}

// --------- TRADUCCIÓN DE DIRECCIÓN --------- //
func traduccionDireccionLogica(nroPagina int, direccionLogica int) int {
	if tlbHabilitada {
		if EstaEnTLB(nroPagina) { // TLB Hit
			slog.Info(fmt.Sprintf("PID: %d - TLB HIT - Pagina: %d", ejecutandoPID, nroPagina)) // log obligatorio

			nroMarcoInt := obtenerMarcoTLB(nroPagina)
			slog.Info(fmt.Sprintf("PID: %d - OBTENER MARCO - Pagina: %d - Marco: %d", ejecutandoPID, nroPagina, nroMarcoInt)) // log obligatorio
			// Actualizar tiempo de referencia de la entrada TLB
			for i := range TLB {
				if TLB[i].NUMERO_PAG == nroPagina {
					TLB[i].TIEMPO_DESDE_REFERENCIA = time.Now() // Actualizar el tiempo de uso de la entrada TLB
				}
			}
			return nroMarcoInt // direccion fisica
		} else { // TLB Miss

			slog.Info(fmt.Sprintf("PID: %d - TLB MISS - Pagina: %d", ejecutandoPID, nroPagina))
			nroMarcoInt := accederAMarco(nroPagina, direccionLogica)
			saveTLB(nroPagina, nroMarcoInt)
			return nroMarcoInt
		}
	} else {
		slog.Debug("TLB DESHABILITADA")
		return accederAMarco(nroPagina, direccionLogica)
	}

}

func accederAMarco(nroPagina int, direccionLogica int) int {

	entrada_nivel_X := MMU(direccionLogica)

	marcoStruct := globales.ObtenerMarco{
		PID:              ejecutandoPID,
		Entradas_Nivel_X: entrada_nivel_X,
	}

	_, nroMarco := globales.GenerarYEnviarPaquete(&marcoStruct, ClientConfig.IP_MEMORY, ClientConfig.PORT_MEMORY, "/cpu/obtener_marco")
	marco, err := io.ReadAll(bytes.NewReader(nroMarco))

	if err != nil {
		slog.Error(fmt.Sprintf("Error al leer el cuerpo de la respuesta: %v", err))
	}
	nroMarcoInt, _ := strconv.Atoi(string(marco))

	slog.Info(fmt.Sprintf("PID: %d - OBTENER MARCO - Pagina: %d - Marco: %d", ejecutandoPID, nroPagina, nroMarcoInt)) // log obligatorio
	return nroMarcoInt
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
	nuevaEntradaTLB := EntradaTLB{
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
		slog.Debug(fmt.Sprintf("Reemplazando entrada TLB por FIFO: Pagina %d, Marco %d", nuevaEntradaTLB.NUMERO_PAG, nuevaEntradaTLB.NUMERO_MARCO))
		TLB = append(TLB[1:], nuevaEntradaTLB) // Reemplazamos siempre la primera entrada
	} else { // LRU: Ver si acomodamos la TLB antes de reemplazar y siempre sacar el primerop
		indiceMenosUsado := 0
		slog.Debug(fmt.Sprintf("Reemplazando entrada TLB por LRU: Pagina %d, Marco %d", nuevaEntradaTLB.NUMERO_PAG, nuevaEntradaTLB.NUMERO_MARCO))
		// el que hace mas tiempo que no se referencia, es el que mas TIEMPO_DESDE_REFERENCIA tiene
		for i, entrada := range TLB {
			if entrada.TIEMPO_DESDE_REFERENCIA.Before(TLB[indiceMenosUsado].TIEMPO_DESDE_REFERENCIA) {
				indiceMenosUsado = i
			}
		}
		slog.Debug(fmt.Sprintf("Reemplazando entrada TLB: %v", TLB[indiceMenosUsado]))
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

func EliminarEntradasTLB() {
	TLB = []EntradaTLB{} // Limpiar TLB
	slog.Debug("Se han eliminado las entradas de la TLB del proceso")
}

// Direcciones logicas
func MMU(direccionLogica int) (entrada_nivel_X []int) {
	nroPagina := direccionLogica / TamanioPagina
	entrada_nivel_X = make([]int, CantidadNiveles)
	for x := 1; x <= CantidadNiveles; x++ {
		divisor := int(math.Pow(float64(CantidadEntradas), float64(CantidadNiveles-x)))
		entrada_nivel_X[x-1] = (nroPagina / divisor) % CantidadEntradas
	}
	return
}

// --------- MEMORIA CACHE --------- //
func buscarEntradaCache(nroPagina int, direccionLogica int) (indiceEntradaCache int) {
	for i := range MemoriaCache {
		if MemoriaCache[i].nroPagina == nroPagina && MemoriaCache[i].entradaOcupada {
			slog.Info(fmt.Sprintf("PID: %d - Cache Hit - Pagina: %d", ejecutandoPID, nroPagina)) // log obligatorio
			MemoriaCache[i].bitDeUso = true                                                      // Actualizamos el bit de uso
			return i
		}
	}
	slog.Info(fmt.Sprintf("PID: %d - Cache Miss - Pagina: %d", ejecutandoPID, nroPagina)) // log obligatorio

	nroMarco := traduccionDireccionLogica(nroPagina, direccionLogica)

	direccionFisica := nroMarco * TamanioPagina // direccion fisica
	peticion := globales.LeerMarcoMemoria{
		DIRECCION: direccionFisica,
	}

	_, contenidoPagina := globales.GenerarYEnviarPaquete(&peticion, ClientConfig.IP_MEMORY, ClientConfig.PORT_MEMORY, "/cpu/leer_pagina")

	return cargarEntradaCache(nroPagina, nroMarco, contenidoPagina) //TODO: pasarle los datos que vienen de memoria
}

func cargarEntradaCache(nroPagina int, nroMarco int, contenidoPagina []byte) (indiceEntradaCache int) {
	for i := range MemoriaCache {
		if !MemoriaCache[i].entradaOcupada {
			MemoriaCache[i].nroPagina = nroPagina
			MemoriaCache[i].Datos = contenidoPagina // Inicializamos los datos como un slice vacio
			MemoriaCache[i].bitDeUso = true
			MemoriaCache[i].bitModificado = false
			MemoriaCache[i].entradaOcupada = true

			MemoriaCache[i].nroMarco = nroMarco                                                  // Guardamos el nro de marco para facilitar la traduccion de direccion logica a fisica
			punteroMemoriaCache = i + 1                                                          // Actualizamos el puntero de la cache
			slog.Info(fmt.Sprintf("PID: %d - Cache Add - Pagina: %d", ejecutandoPID, nroPagina)) // log obligatorio
			return i
		}
	}
	return remplazarEntradaCache(nroPagina, nroMarco, contenidoPagina)

}

func remplazarEntradaCache(nroPagina int, nroMarco int, contenidoPagina []byte) (indiceEntradaCache int) {
	if algoritmoCache == "CLOCK" {
		slog.Debug(fmt.Sprintf("Reemplazando entrada de cache por CLOCK: Pagina %d", nroPagina))
		for {
			for i := punteroMemoriaCache; i < len(MemoriaCache); i++ {
				if !MemoriaCache[i].bitDeUso && MemoriaCache[i].entradaOcupada {
					slog.Debug(fmt.Sprintf("Reemplazando entrada de cache: Pagina %d, Entrada %d", MemoriaCache[i].nroPagina, i))

					if MemoriaCache[i].bitModificado {
						escribirPaginaCacheEnMemoria(i)
					}

					MemoriaCache[i].nroPagina = nroPagina
					MemoriaCache[i].Datos = contenidoPagina // Inicializamos los datos como un slice vacio
					MemoriaCache[i].bitDeUso = true
					MemoriaCache[i].bitModificado = false

					MemoriaCache[i].nroMarco = nroMarco                                                  // Guardamos el nro de marco para facilitar la traduccion de direccion logica a fisica
					punteroMemoriaCache = i + 1                                                          // Actualizamos el puntero de la cache
					slog.Info(fmt.Sprintf("PID: %d - Cache Add - Pagina: %d", ejecutandoPID, nroPagina)) // log obligatorio
					return i
				} else {

					slog.Debug(fmt.Sprintf("Entrada de cache: Pagina %d, Entrada %d, Bit de uso: %t", MemoriaCache[i].nroPagina, i, MemoriaCache[i].bitDeUso))
					MemoriaCache[i].bitDeUso = false // Reiniciamos el bit de uso

				}
			}
			for i := 0; i < punteroMemoriaCache; i++ {
				if !MemoriaCache[i].bitDeUso && MemoriaCache[i].entradaOcupada {
					slog.Debug(fmt.Sprintf("Reemplazando entrada de cache: Pagina %d, Entrada %d", MemoriaCache[i].nroPagina, i))

					if MemoriaCache[i].bitModificado {
						escribirPaginaCacheEnMemoria(i)
					}

					MemoriaCache[i].nroPagina = nroPagina
					MemoriaCache[i].Datos = contenidoPagina
					MemoriaCache[i].bitDeUso = true
					MemoriaCache[i].bitModificado = false

					MemoriaCache[i].nroMarco = nroMarco // Guardamos el nro de marco para facilitar la traduccion de direccion logica a fisica
					punteroMemoriaCache = i + 1
					slog.Info(fmt.Sprintf("PID: %d - Cache Add - Pagina: %d", ejecutandoPID, nroPagina)) // log obligatorio
					return i
				} else {
					slog.Debug(fmt.Sprintf("Entrada de cache: Pagina %d, Entrada %d, Bit de uso: %t", MemoriaCache[i].nroPagina, i, MemoriaCache[i].bitDeUso))
					MemoriaCache[i].bitDeUso = false // Reiniciamos el bit de uso
				}
			}
		}
	}
	if algoritmoCache == "CLOCK-M" {
		for {
			for i := punteroMemoriaCache; i < len(MemoriaCache); i++ {
				if MemoriaCache[i].entradaOcupada && !MemoriaCache[i].bitDeUso && !MemoriaCache[i].bitModificado {
					slog.Debug(fmt.Sprintf("Reemplazando entrada de cache: Pagina %d, Entrada %d, Bit de uso: %t, Bit modificado: %t", MemoriaCache[i].nroPagina, i, MemoriaCache[i].bitDeUso, MemoriaCache[i].bitModificado))

					MemoriaCache[i].nroPagina = nroPagina
					MemoriaCache[i].Datos = contenidoPagina
					MemoriaCache[i].bitDeUso = true
					MemoriaCache[i].bitModificado = false

					MemoriaCache[i].nroMarco = nroMarco                                                  // Guardamos el nro de marco para facilitar la traduccion de direccion logica a fisica
					punteroMemoriaCache = i + 1                                                          // Actualizamos el puntero de la cache
					slog.Info(fmt.Sprintf("PID: %d - Cache Add - Pagina: %d", ejecutandoPID, nroPagina)) // log obligatorio
					return i
				}
			}
			for i := 0; i < punteroMemoriaCache; i++ {
				if MemoriaCache[i].entradaOcupada && !MemoriaCache[i].bitDeUso && !MemoriaCache[i].bitModificado {
					slog.Debug(fmt.Sprintf("Reemplazando entrada de cache: Pagina %d, Entrada %d, Bit de uso: %t, Bit modificado: %t", MemoriaCache[i].nroPagina, i, MemoriaCache[i].bitDeUso, MemoriaCache[i].bitModificado))

					MemoriaCache[i].nroPagina = nroPagina
					MemoriaCache[i].Datos = contenidoPagina
					MemoriaCache[i].bitDeUso = true
					MemoriaCache[i].bitModificado = false

					MemoriaCache[i].nroMarco = nroMarco                                                  // Guardamos el nro de marco para facilitar la traduccion de direccion logica a fisica
					punteroMemoriaCache = i + 1                                                          // Actualizamos el puntero de la cache
					slog.Info(fmt.Sprintf("PID: %d - Cache Add - Pagina: %d", ejecutandoPID, nroPagina)) // log obligatorio
					return i
				}
			}

			for i := punteroMemoriaCache; i < len(MemoriaCache); i++ {
				if MemoriaCache[i].entradaOcupada && !MemoriaCache[i].bitDeUso && MemoriaCache[i].bitModificado {
					slog.Debug(fmt.Sprintf("Reemplazando entrada de cache: Pagina %d, Entrada %d, Bit de uso: %t, Bit modificado: %t", MemoriaCache[i].nroPagina, i, MemoriaCache[i].bitDeUso, MemoriaCache[i].bitModificado))

					escribirPaginaCacheEnMemoria(i)

					MemoriaCache[i].nroPagina = nroPagina
					MemoriaCache[i].Datos = contenidoPagina
					MemoriaCache[i].bitDeUso = true
					MemoriaCache[i].bitModificado = false

					MemoriaCache[i].nroMarco = nroMarco                                                  // Guardamos el nro de marco para facilitar la traduccion de direccion logica a fisica
					punteroMemoriaCache = i + 1                                                          // Actualizamos el puntero de la cache
					slog.Info(fmt.Sprintf("PID: %d - Cache Add - Pagina: %d", ejecutandoPID, nroPagina)) // log obligatorio
					return i
				} else {
					slog.Info(fmt.Sprintf("Entrada de cache: Pagina %d, Entrada %d, Bit de uso: %t, Bit modificado: %t", MemoriaCache[i].nroPagina, i, MemoriaCache[i].bitDeUso, MemoriaCache[i].bitModificado))
					MemoriaCache[i].bitDeUso = false // Reiniciamos el bit de uso
				}
			}
			for i := 0; i < punteroMemoriaCache; i++ {
				if MemoriaCache[i].entradaOcupada && !MemoriaCache[i].bitDeUso && MemoriaCache[i].bitModificado {
					slog.Debug(fmt.Sprintf("Reemplazando entrada de cache: Pagina %d, Entrada %d, Bit de uso: %t, Bit modificado: %t", MemoriaCache[i].nroPagina, i, MemoriaCache[i].bitDeUso, MemoriaCache[i].bitModificado))

					escribirPaginaCacheEnMemoria(i)

					MemoriaCache[i].nroPagina = nroPagina
					MemoriaCache[i].Datos = contenidoPagina
					MemoriaCache[i].bitDeUso = true
					MemoriaCache[i].bitModificado = false

					MemoriaCache[i].nroMarco = nroMarco                                                  // Guardamos el nro de marco para facilitar la traduccion de direccion logica a fisica
					punteroMemoriaCache = i + 1                                                          // Actualizamos el puntero de la cache
					slog.Info(fmt.Sprintf("PID: %d - Cache Add - Pagina: %d", ejecutandoPID, nroPagina)) // log obligatorio
					return i
				} else {
					slog.Debug(fmt.Sprintf("Entrada de cache: Pagina %d, Entrada %d, Bit de uso: %t, Bit modificado: %t", MemoriaCache[i].nroPagina, i, MemoriaCache[i].bitDeUso, MemoriaCache[i].bitModificado))
					MemoriaCache[i].bitDeUso = false // Reiniciamos el bit de uso
				}
			}
		}
	}

	return -1 // Me pide un return pero nunca deberia llegar a este punto
}

func escribirPaginaCacheEnMemoria(indiceEntradaCache int) {
	if MemoriaCache[indiceEntradaCache].bitModificado {
		direccionFisica := MemoriaCache[indiceEntradaCache].nroMarco * TamanioPagina

		peticion := globales.EscribirMarcoMemoria{
			DIRECCION: direccionFisica,
			PID:       ejecutandoPID,
			DATOS:     MemoriaCache[indiceEntradaCache].Datos,
		}

		resp, _ := globales.GenerarYEnviarPaquete(&peticion, ClientConfig.IP_MEMORY, ClientConfig.PORT_MEMORY, "/cpu/escribir_pagina")
		if resp.StatusCode != http.StatusOK {
			slog.Error(fmt.Sprintf("Error al escribir en memoria: %s", resp.Status))
			return
		} else {
			slog.Info(fmt.Sprintf("PID: %d - Memory Update - Página: %d - Frame: %d", ejecutandoPID, MemoriaCache[indiceEntradaCache].nroPagina, MemoriaCache[indiceEntradaCache].nroMarco)) // log obligatorio
		}

	}
}

func limpiarCache() {
	for i := range MemoriaCache {
		if MemoriaCache[i].bitModificado {
			escribirPaginaCacheEnMemoria(i)
		}
		MemoriaCache[i] = EntradaCache{
			nroPagina:      -1,                          // Inicializamos con -1 para indicar que no hay pagina cargada
			Datos:          make([]byte, TamanioPagina), // Inicializamos los datos como un slice vacio
			nroMarco:       -1,                          // Inicializamos con -1 para indicar que no hay marco cargado
			bitDeUso:       false,
			bitModificado:  false,
			entradaOcupada: false,
		}
	}
	punteroMemoriaCache = 0 // Reiniciamos el puntero de la cache
	slog.Debug("Se ha limpiado la cache del CPU")
}
