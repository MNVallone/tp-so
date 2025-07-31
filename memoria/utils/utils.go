package utils

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"globales"
	"io"
	"log"
	"log/slog"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// --------- VARIABLES DE MEMORIA --------- //
var RutaModulo string // Ruta del modulo memoria

var ClientConfig *Config

// mapa de instrucciones por PID
var instruccionesProcesos = make(map[int]map[int]string)

var mutexInstrucciones sync.Mutex // Mutex para proteger el acceso al mapa de instrucciones

var MetricasPorProceso = make(map[int]METRICAS_PROCESO) // Mapa de metricas por PID
// var mutexMetricas sync.Mutex

var MemoriaDeUsuario []byte // Simulacion de la memoria de usuario
var MarcosLibres []int

var mutexMemoria sync.Mutex            // Mutex para proteger el acceso a la memoria de usuario
var mutexMetricasPorProceso sync.Mutex // Mutex para proteger el acceso a las metricas de los procesos

var ProcesosEnMemoria []*Proceso
var mutexProcesosEnMemoria sync.Mutex // Mutex para proteger el acceso a la lista de procesos en memoria

var mutexArchivoSwap sync.Mutex // Mutex para proteger el acceso al archivo de swap

// --------- ESTRUCTURAS DE MEMORIA --------- //
type Config struct {
	PORT_MEMORY      int    `json:"port_memory"`
	IP_MEMORY        string `json:"ip_memory"`
	MEMORY_SIZE      int    `json:"memory_size"`
	PAGE_SIZE        int    `json:"page_size"`
	ENTRIES_PER_PAGE int    `json:"entries_per_page"`
	NUMBER_OF_LEVELS int    `json:"number_of_levels"`
	MEMORY_DELAY     int    `json:"memory_delay"`
	SWAPFILE_PATH    string `json:"swapfile_path"`
	SWAP_DELAY       int    `json:"swap_delay"`
	LOG_LEVEL        string `json:"log_level"`
	DUMP_PATH        string `json:"dump_path"`
	SCRIPTS_PATH     string `json:"scripts_path"`
}

// Para la memoria, un proceso se reduce a su ID y su Tabla de Paginas.
type Proceso struct {
	PID          int
	TablaPaginas *NodoTablaPaginas
	Suspendido   chan int
}

type ProcesoSwap struct {
	PID  int
	Data []byte
}

type METRICAS_PROCESO struct { //Cuando se reserva espacio en memoria inicializamos esta estructura
	CANT_ACCESOS_TABLA_DE_PAGINAS  int `json:"cant_accesos_tabla_de_paginas"`
	CANT_INSTRUCCIONES_SOLICITADAS int `json:"cant_instrucciones_solicitadas"`
	CANT_BAJADAS_A_SWAP            int `json:"cant_accesos_swap"`
	CANT_SUBIDAS_A_MEMORIA         int `json:"cant_subidas_a_memoria"`
	CANT_LECTURAS_MEMORIA          int `json:"cant_lecturas_memoria"`
	CANT_ESCRITURAS_MEMORIA        int `json:"cant_escrituras_memoria"`
}

type NodoTablaPaginas struct {
	Children []*NodoTablaPaginas // Para niveles intermedios
	Marcos   []*int
}

// --------- INICIO DE MEMORIA FISICA --------- //
func InicializarMemoria() {
	// Creo la memoria de usuario
	MemoriaDeUsuario = make([]byte, ClientConfig.MEMORY_SIZE)

	// Divido la memoria en marcos
	var cant_paginas int = ClientConfig.MEMORY_SIZE / ClientConfig.PAGE_SIZE
	MarcosLibres = make([]int, cant_paginas)
	for idx := range MarcosLibres {
		MarcosLibres[idx] = idx
	}

	// Creacion del archivo de SWAP
	err := os.WriteFile(RutaModulo+ClientConfig.SWAPFILE_PATH, []byte{}, 0644)
	if err != nil {
		panic(err)
	}

}

// --------- FUNCIONES AUXILIARES --------- //
func IniciarConfiguracion(filePath string) *Config {
	var config *Config
	configFile, err := os.Open(filePath)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer configFile.Close()

	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&config)

	slog.Debug("Configuración de memoria cargada correctamente", "config", config)

	return config
}

func delayDeMemoria() {
	time.Sleep(time.Duration(ClientConfig.MEMORY_DELAY) * time.Millisecond) // Simula el delay de acceso a memoria
}

func delayDeSwap() {
	time.Sleep(time.Duration(ClientConfig.SWAP_DELAY) * time.Millisecond) // Simula el delay de acceso a swap
}

func LeerArchivoDePseudocodigo(rutaArchivo string, pid int) {
	filePath := filepath.Join(ClientConfig.SCRIPTS_PATH, rutaArchivo)
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("Error al abrir el archivo '%s': %v", filePath, err)
	}
	// Asegurar que el archivo se cierre al final de la función main
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNumber := 0

	// Inicializo el mapa de instrucciones para el proceso
	mutexInstrucciones.Lock()
	if instruccionesProcesos[pid] == nil {
		instruccionesProcesos[pid] = make(map[int]string)
	}
	mutexInstrucciones.Unlock()

	// Leo el archivo línea por línea
	for scanner.Scan() {
		lineText := scanner.Text()

		// Guardo en el mapa: PID, valor = texto de la línea
		mutexInstrucciones.Lock()
		instruccionesProcesos[pid][lineNumber] = lineText
		mutexInstrucciones.Unlock()

		lineNumber++
	}

	// 7. Verificar si hubo errores durante el escaneo (distintos a EOF)
	if err := scanner.Err(); err != nil {
		log.Fatalf("Error al leer el archivo '%s': %v", rutaArchivo, err)
	}

}

// --------- HANDLERS DEL CPU --------- //
func AtenderHandshakeCPU(w http.ResponseWriter, r *http.Request) {

	respuesta := globales.ParametrosMemoria{
		CantidadEntradas: ClientConfig.ENTRIES_PER_PAGE,
		TamanioPagina:    ClientConfig.PAGE_SIZE,
		CantidadNiveles:  ClientConfig.NUMBER_OF_LEVELS,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(respuesta)
}

func DevolverInstruccion(w http.ResponseWriter, r *http.Request) {
	paquete := globales.PeticionInstruccion{}
	paquete = globales.DecodificarPaquete(w, r, &paquete)

	delayDeMemoria()

	pidString := strconv.Itoa(paquete.PID)
	pcString := strconv.Itoa(paquete.PC)

	// Buscar instruccion
	slog.Debug(fmt.Sprintf("Buscando instrucción en memoria para PC: %s", pcString))

	instruccion, err := json.Marshal(instruccionesProcesos[paquete.PID][paquete.PC])
	if err != nil {
		http.Error(w, "Error al codificar los datos como JSON", http.StatusInternalServerError)
		return
	}

	slog.Info(fmt.Sprintf("## PID %s - Obtener Instruccion: %s - Instruccion: %s", pidString, pcString, instruccion)) // log obligatorio

	mutexMetricasPorProceso.Lock()
	metricas := MetricasPorProceso[paquete.PID]
	metricas.CANT_INSTRUCCIONES_SOLICITADAS += 1
	MetricasPorProceso[paquete.PID] = metricas
	mutexMetricasPorProceso.Unlock()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(instruccion))
}

func LeerDireccion(w http.ResponseWriter, r *http.Request) {
	paquete := globales.LeerMemoria{}
	paquete = globales.DecodificarPaquete(w, r, &paquete)

	delayDeMemoria()
	respuesta := make([]byte, paquete.TAMANIO)

	// Si me llega un byte que es multiplo del tamaño de la pagina, leo la pagina completa
	mutexMemoria.Lock()
	for i := 0; i < paquete.TAMANIO; i++ {
		respuesta[i] = MemoriaDeUsuario[paquete.DIRECCION+i]
	}
	mutexMemoria.Unlock()

	slog.Info(fmt.Sprintf("## PID: %d - Lectura - Dir.Física: %d - Tamaño: %v", paquete.PID, paquete.DIRECCION, paquete.TAMANIO)) // log obligatorio

	mutexMetricasPorProceso.Lock()
	metricas := MetricasPorProceso[paquete.PID]
	metricas.CANT_LECTURAS_MEMORIA += 1
	MetricasPorProceso[paquete.PID] = metricas
	mutexMetricasPorProceso.Unlock()

	w.WriteHeader(http.StatusOK)
	w.Write(respuesta)
}

func EscribirDireccion(w http.ResponseWriter, r *http.Request) {
	paquete := globales.EscribirMemoria{}
	paquete = globales.DecodificarPaquete(w, r, &paquete)
	delayDeMemoria()

	informacion := []byte(paquete.DATOS)

	mutexMemoria.Lock()
	for i := 0; i < len(informacion); i++ {
		MemoriaDeUsuario[paquete.DIRECCION+i] = informacion[i]
	}
	mutexMemoria.Unlock()

	slog.Info(fmt.Sprintf("## PID: %d - Escritura - Dir.Física: %d - Tamaño: %v", paquete.PID, paquete.DIRECCION, len(paquete.DATOS))) // log obligatorio

	mutexMetricasPorProceso.Lock()
	metricas := MetricasPorProceso[paquete.PID]
	metricas.CANT_ESCRITURAS_MEMORIA += 1
	MetricasPorProceso[paquete.PID] = metricas
	mutexMetricasPorProceso.Unlock()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func DumpearProceso(w http.ResponseWriter, r *http.Request) {
	paquete := globales.PID{}
	paquete = globales.DecodificarPaquete(w, r, &paquete)

	slog.Info(fmt.Sprintf("## PID: %d - Memory Dump solicitado", paquete.NUMERO_PID)) // log obligatorio

	delayDeMemoria()

	proceso, err := ObtenerProceso(paquete.NUMERO_PID)
	if err != nil {
		slog.Error(fmt.Sprintf("Error buscando el proceso, %v", err))
	}

	<-proceso.Suspendido
	slog.Debug("CHANNEL SUSPENDIDO-DUMPEAR PROCESO (-1) ")

	buffer := new(bytes.Buffer)
	encoder := gob.NewEncoder(buffer)

	datosProceso := ConcatenarDatosProceso(paquete.NUMERO_PID)
	slog.Debug(fmt.Sprintf("datos proceso (DUMP): %v", datosProceso))
	encoder.Encode(datosProceso)
	data := buffer.Bytes()
	slog.Debug(fmt.Sprintf("Buffer bytes (DUMP): %v", data))

	nombreArchivo := fmt.Sprintf("%d-%d.dmp", paquete.NUMERO_PID, time.Now().Unix())
	rutadmp := filepath.Join(ClientConfig.DUMP_PATH, nombreArchivo)

	// Aseguro que exista el directorio
	if err := os.MkdirAll(ClientConfig.DUMP_PATH, 0755); err != nil {
		slog.Error(fmt.Sprintf("Error creando directorio dump: %v", err))
		return
	}

	file, err := os.Create(rutadmp)
	if err != nil {
		slog.Error(fmt.Sprintf("Error creando archivo dump: %v", err))
		return
	}
	defer file.Close()
	_, errWrite := file.Write(data) // data es []byte
	if errWrite != nil {
		panic(errWrite)
	}

	proceso.Suspendido <- 1
	slog.Debug("channel suspendido (dump + 1)")

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// --------- HANDLERS DEL KERNEL --------- //
func InicializarProceso(w http.ResponseWriter, r *http.Request) {
	var peticion globales.MEMORIA_CREACION_PROCESO
	peticion = globales.DecodificarPaquete(w, r, &peticion)

	delayDeMemoria()

	// 1. Creo la tabla de paginas del proceso y la guardo.
	TablaDePaginas := CrearTablaPaginas(1, ClientConfig.NUMBER_OF_LEVELS, ClientConfig.ENTRIES_PER_PAGE)

	// 2. Le asigno el espacio solicitado (si es posible)
	asignado := ReservarMemoria(peticion.Tamanio, TablaDePaginas)

	if !asignado {
		w.WriteHeader(http.StatusInsufficientStorage)
		w.Write([]byte("No se pudo asignar la memoria solicitada."))
		return
	}

	// 3. Creo el proceso y lo guardo en la lista de procesos en memoria
	nuevoProceso := Proceso{
		PID:          peticion.PID,
		TablaPaginas: TablaDePaginas,
		Suspendido:   make(chan int, 1),
	}
	nuevoProceso.Suspendido <- 1

	mutexProcesosEnMemoria.Lock()
	ProcesosEnMemoria = append(ProcesosEnMemoria, &nuevoProceso)
	mutexProcesosEnMemoria.Unlock()

	mutexMetricasPorProceso.Lock()
	MetricasPorProceso[peticion.PID] = METRICAS_PROCESO{}
	mutexMetricasPorProceso.Unlock()

	slog.Info(fmt.Sprintf("## PID: %d - Proceso Creado - Tamaño: %d", peticion.PID, peticion.Tamanio)) // log obligatorio

	// 4. Cargar el archivo de pseudocodigo
	LeerArchivoDePseudocodigo(peticion.RutaArchivoPseudocodigo, peticion.PID)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func FinalizarProceso(w http.ResponseWriter, r *http.Request) {
	paquete := globales.PID{}
	paquete = globales.DecodificarPaquete(w, r, &paquete)

	found := false

	delayDeMemoria()

	slog.Debug(fmt.Sprintf("Procesos en memoria al inicio de la funcion: %v", ProcesosEnMemoria))

	for i, p := range ProcesosEnMemoria {
		if p.PID == paquete.NUMERO_PID {
			found = true
			slog.Debug("Proceso encontrado en ProcesosEnMemoria")
			// Desasignar marcos de memoria
			DesasignarMarcos(p.TablaPaginas, 1)
			ProcesosEnMemoria = remove(ProcesosEnMemoria, i)
			if _, err := buscarProcesoEnSwap(p.PID); err == nil {
				slog.Debug(fmt.Sprintf("## PID: %d - Proceso encontrado en Swap, borrando datos de swap", p.PID))
				borrarEntradaDeSwap(*p)
			}
			slog.Debug(fmt.Sprintf("Proceso con PID %d destruido exitosamente.", paquete.NUMERO_PID))
			break
		}
	}

	if !found {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("No se encontro el proceso solicitado."))
		return
	}

	MostrarMetricasProceso(paquete.NUMERO_PID)

	mutexMetricasPorProceso.Lock()
	delete(MetricasPorProceso, paquete.NUMERO_PID)
	mutexMetricasPorProceso.Unlock()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Proceso eliminado con exito."))
}

func MostrarMetricasProceso(pid int) {
	mutexMetricasPorProceso.Lock()
	metricas, existe := MetricasPorProceso[pid]
	mutexMetricasPorProceso.Unlock()
	if !existe {
		slog.Error(fmt.Sprintf("No existen métricas para el PID %d\n", pid))
		return
	}
	slog.Info(fmt.Sprintf("## PID: %d - Proceso Destruido - Métricas - Acc.T.Pag: %d; Inst.Sol.: %d; SWAP: %d; Mem.Prin.: %d; Lec.Mem.: %d; Esc.Mem.: %d", pid, metricas.CANT_ACCESOS_TABLA_DE_PAGINAS, metricas.CANT_INSTRUCCIONES_SOLICITADAS, metricas.CANT_BAJADAS_A_SWAP, metricas.CANT_SUBIDAS_A_MEMORIA, metricas.CANT_LECTURAS_MEMORIA, metricas.CANT_ESCRITURAS_MEMORIA))
}

func ObtenerMarco(w http.ResponseWriter, r *http.Request) {
	paquete := globales.ObtenerMarco{}
	paquete = globales.DecodificarPaquete(w, r, &paquete)

	// Obtener el marco de memoria correspondiente
	mutexProcesosEnMemoria.Lock()
	var marco int = -1
	for _, proceso := range ProcesosEnMemoria {
		if proceso.PID == paquete.PID {
			marco = ObtenerMarcoDeTDP(paquete.PID, proceso.TablaPaginas, paquete.Entradas_Nivel_X, 1)
			break
		}
	}
	mutexProcesosEnMemoria.Unlock()

	if marco == -1 {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("No se encontro el marco solicitado."))
		return
	}

	slog.Debug(fmt.Sprintf("PID: %d - Marco obtenido: %d", paquete.PID, marco))
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(strconv.Itoa(marco)))
}

func remove(s []*Proceso, i int) []*Proceso {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}

// --------- PAGINACION MULTINIVEL --------- //
func CrearTablaPaginas(semilla, numNiveles, entradasPorPagina int) *NodoTablaPaginas {
	nodo := &NodoTablaPaginas{}
	if semilla < numNiveles { // si es una tabla intermedia
		nodo.Children = make([]*NodoTablaPaginas, entradasPorPagina)
		for i := 0; i < entradasPorPagina; i++ {
			nodo.Children[i] = CrearTablaPaginas(semilla+1, numNiveles, entradasPorPagina)
		}
	} else { // si es la de ultimo nivel
		nodo.Children = nil
		nodo.Marcos = make([]*int, entradasPorPagina)
	}
	return nodo
}

func ReservarMemoria(tamanioProceso int, TablaPaginas *NodoTablaPaginas) bool {
	div := float64(tamanioProceso) / float64(ClientConfig.PAGE_SIZE)
	cant_paginas_proceso := int(math.Ceil(float64(div)))

	// El tamanio de los procesos esta limitado por el esquema de paginacion
	maximoTamPorProceso := int(float64(ClientConfig.PAGE_SIZE) * math.Pow(float64(ClientConfig.ENTRIES_PER_PAGE), float64(ClientConfig.NUMBER_OF_LEVELS)))

	mutexMemoria.Lock()

	if len(MarcosLibres) < cant_paginas_proceso || tamanioProceso > maximoTamPorProceso {
		mutexMemoria.Unlock()
		slog.Error("No hay suficientes paginas para almacenar el proceso completo en memoria")
		return false
	}

	AsignarMarcos(TablaPaginas, 1, &cant_paginas_proceso)

	mutexMemoria.Unlock()

	return true
}

// Asigna marcos libres a las hojas que no estén ocupadas
func AsignarMarcos(node *NodoTablaPaginas, level int, marcosRestantes *int) {
	if *marcosRestantes > 0 { // ¿Quedan marcos por cargar?
		if level == ClientConfig.NUMBER_OF_LEVELS {

			for i := range node.Marcos {
				if *marcosRestantes > 0 {
					node.Marcos[i] = &MarcosLibres[0]
					slog.Debug(fmt.Sprintf("\n Asignando marco %d a la entrada %d del nivel %d, valor puntero: %v", *node.Marcos[i], i, level, node.Marcos[i]))
					MarcosLibres = MarcosLibres[1:]
					slog.Debug(fmt.Sprintf("\n Longitud de marcos libres %d", len(MarcosLibres)))
					nuevosMarcos := *marcosRestantes - 1
					*marcosRestantes = nuevosMarcos
				}
			}

		} else { // No es el último nivel
			for i := 0; i < ClientConfig.ENTRIES_PER_PAGE; i++ {
				slog.Debug(fmt.Sprintf("\n Asignando pagina %v a la entrada %d del nivel %d, valor puntero: %v", *node.Children[i], i, level, node.Children[i]))
				AsignarMarcos(node.Children[i], level+1, marcosRestantes)
			}
		}
	}
}

func DesasignarMarcos(node *NodoTablaPaginas, level int) {
	// ¿Quedan marcos por cargar?
	if level == ClientConfig.NUMBER_OF_LEVELS {
		for i := range node.Marcos {
			if node.Marcos[i] == nil {
				return
			}
			slog.Debug(fmt.Sprintf("\n Desasignado el marco: %d", *node.Marcos[i]))
			MarcosLibres = append(MarcosLibres, *node.Marcos[i]) // Agrega el marco liberado
			node.Marcos[i] = nil                                 // Limpia la referencia al marco
		}

	} else { // No es el último nivel
		for i := 0; i < ClientConfig.ENTRIES_PER_PAGE; i++ {
			DesasignarMarcos(node.Children[i], level+1)
		}
	}

}

func ObtenerMarcoDeTDP(PID int, TDP *NodoTablaPaginas, entrada_nivel_X []int, level int) int {
	slog.Debug(fmt.Sprintf("Obteniendo marco de TDP. Nivel: %d, Entradas: %v ...", level, entrada_nivel_X))
	delayDeMemoria() // Simula el delay de acceso a memoria
	slog.Debug(fmt.Sprintf("Accediendo a TDP de nivel %d, Contenido: %v", level, *TDP))

	mutexMetricasPorProceso.Lock()
	metricas := MetricasPorProceso[PID]
	metricas.CANT_ACCESOS_TABLA_DE_PAGINAS += 1
	MetricasPorProceso[PID] = metricas
	mutexMetricasPorProceso.Unlock()

	if level == ClientConfig.NUMBER_OF_LEVELS {
		slog.Debug(fmt.Sprintf("Accediendo a direccion: %d", *TDP.Marcos[entrada_nivel_X[ClientConfig.NUMBER_OF_LEVELS-1]]))
		numeroMarco := *TDP.Marcos[entrada_nivel_X[ClientConfig.NUMBER_OF_LEVELS-1]]
		return numeroMarco // Retorna el marco de memoria al que se accede
	} else {
		return ObtenerMarcoDeTDP(PID, TDP.Children[entrada_nivel_X[level-1]], entrada_nivel_X, level+1) // Accede al siguiente nivel
	}
}

func ObtenerMarcosAsignados(PID int, node *NodoTablaPaginas, level int, marcosAsignados *[]int) {
	delayDeMemoria() // Simula el delay de acceso a memoria

	mutexMetricasPorProceso.Lock()
	metricas := MetricasPorProceso[PID]
	metricas.CANT_ACCESOS_TABLA_DE_PAGINAS += 1
	MetricasPorProceso[PID] = metricas
	mutexMetricasPorProceso.Unlock()

	if level == ClientConfig.NUMBER_OF_LEVELS {

		for i := range node.Marcos {
			if node.Marcos[i] == nil {
				return
			}
			slog.Debug(fmt.Sprintf("\nEntrada numero %d: %d", i, *node.Marcos[i]))
			*marcosAsignados = append(*marcosAsignados, *node.Marcos[i])
		}
	} else {
		for i := 0; i < ClientConfig.ENTRIES_PER_PAGE; i++ {
			slog.Debug(fmt.Sprintf("\nAccediendo a la %dº a TDP de nivel %d", i+1, level+1))
			ObtenerMarcosAsignados(PID, node.Children[i], level+1, marcosAsignados)
		}
	}
}

func LeerPaginaCompleta(w http.ResponseWriter, r *http.Request) {
	paquete := globales.LeerMarcoMemoria{}
	paquete = globales.DecodificarPaquete(w, r, &paquete)

	delayDeMemoria()

	direccion := paquete.DIRECCION
	desplazamiento := (direccion + ClientConfig.PAGE_SIZE)

	mutexMemoria.Lock()
	memoriaLeida := MemoriaDeUsuario[direccion:desplazamiento]
	mutexMemoria.Unlock()

	mutexMetricasPorProceso.Lock()
	metricas := MetricasPorProceso[paquete.PID]
	metricas.CANT_LECTURAS_MEMORIA += 1
	MetricasPorProceso[paquete.PID] = metricas
	mutexMetricasPorProceso.Unlock()

	slog.Info(fmt.Sprintf("## PID: %d - Lectura - Dir.Física: %d - Tamaño: %v", paquete.PID, paquete.DIRECCION, ClientConfig.PAGE_SIZE))

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(memoriaLeida))
}

func EscribirPaginaCompleta(w http.ResponseWriter, r *http.Request) {
	paquete := globales.EscribirMarcoMemoria{}
	paquete = globales.DecodificarPaquete(w, r, &paquete)

	delayDeMemoria()

	mutexMemoria.Lock()
	for i := 0; i < len(paquete.DATOS); i++ {
		MemoriaDeUsuario[paquete.DIRECCION+i] = paquete.DATOS[i]
	}
	mutexMemoria.Unlock()

	mutexMetricasPorProceso.Lock()
	metricas := MetricasPorProceso[paquete.PID]
	metricas.CANT_ESCRITURAS_MEMORIA += 1
	MetricasPorProceso[paquete.PID] = metricas
	mutexMetricasPorProceso.Unlock()

	slog.Info(fmt.Sprintf("## PID: %d - Escritura - Dir.Física: %d - Tamaño: %v", paquete.PID, paquete.DIRECCION, len(paquete.DATOS)))

	w.WriteHeader(http.StatusOK)
}

// Toma la tabla de paginas de un proceso y escribe todos los datos en los marcos asignados, sobreescribiendo la informacion previa.
func EscribirTablaPaginas(procesoMemoria *Proceso, datos []byte) bool {
	var marcosEscritura []int

	mutexMemoria.Lock()
	ObtenerMarcosAsignados(procesoMemoria.PID, procesoMemoria.TablaPaginas, 1, &marcosEscritura)
	contador := 0
	for _, marco := range marcosEscritura {
		datosEscritura := datos[contador : contador+ClientConfig.PAGE_SIZE]
		copy(MemoriaDeUsuario[(marco*ClientConfig.PAGE_SIZE):], datosEscritura)
		slog.Debug(fmt.Sprintf("Marco escrito: %d , datos: %v", marco, datosEscritura))
		contador += ClientConfig.PAGE_SIZE
	}
	mutexMemoria.Unlock()

	return true
}

func EscribirProcesoSwap(file *os.File, proceso ProcesoSwap) error {
	// Escribo el PID
	if err := binary.Write(file, binary.LittleEndian, int32(proceso.PID)); err != nil {
		return err
	}

	// Escribo la longitud del Data
	dataLen := int32(len(proceso.Data))
	if err := binary.Write(file, binary.LittleEndian, dataLen); err != nil {
		return err
	}

	// Escribo el contenido de Data
	_, err := file.Write(proceso.Data)
	return err
}

func LeerProcesosSwap(file *os.File) ([]ProcesoSwap, error) {
	var procesos []ProcesoSwap

	for {
		var pid int32
		err := binary.Read(file, binary.LittleEndian, &pid)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error leyendo PID: %v", err)
		}

		var dataLen int32
		if err := binary.Read(file, binary.LittleEndian, &dataLen); err != nil {
			return nil, fmt.Errorf("error leyendo tamaño: %v", err)
		}

		data := make([]byte, dataLen)
		_, err = io.ReadFull(file, data)
		if err != nil {
			return nil, fmt.Errorf("error leyendo data: %v", err)
		}

		procesos = append(procesos, ProcesoSwap{
			PID:  int(pid),
			Data: data,
		})
	}

	return procesos, nil
}

func SuspenderProceso(w http.ResponseWriter, r *http.Request) {
	paquete := globales.PID{}
	paquete = globales.DecodificarPaquete(w, r, &paquete)
	slog.Debug(fmt.Sprintf("Proceso a swapear: %d", paquete.NUMERO_PID))
	procesoMemoria, err := ObtenerProceso(paquete.NUMERO_PID)
	<-procesoMemoria.Suspendido
	slog.Debug("channel suspendido (suspender - 1)")

	if err != nil {
		procesoMemoria.Suspendido <- 1
		slog.Error(fmt.Sprintf("No se encontro el proceso en la memoria. PID %d: %v", paquete.NUMERO_PID, err))
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("No se encontro el proceso en la memoria."))
		return
	}

	delayDeSwap()

	datosProceso := ConcatenarDatosProceso(paquete.NUMERO_PID)
	slog.Debug(fmt.Sprintf("Datos del proceso %d: %v", paquete.NUMERO_PID, datosProceso))

	procesoASuspeder := ProcesoSwap{
		PID:  paquete.NUMERO_PID,
		Data: datosProceso,
	}

	mutexArchivoSwap.Lock()
	rutaSwap := filepath.Join(RutaModulo, ClientConfig.SWAPFILE_PATH)
	file, err := os.OpenFile(rutaSwap, os.O_APPEND|os.O_RDWR, 0644)

	if err != nil {
		procesoMemoria.Suspendido <- 1
		slog.Error(fmt.Sprintf("Hubo un error con el archivo de swap: %v", err))
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Hubo un error con el archivo de swap."))
		mutexArchivoSwap.Unlock()
		return
	}

	EscribirProcesoSwap(file, procesoASuspeder)

	//DebugSwapCompleto

	file.Close()

	mutexArchivoSwap.Unlock()
	slog.Debug("Archivo de swap escrito.")

	DesasignarMarcos(procesoMemoria.TablaPaginas, 1)
	procesoMemoria.Suspendido <- 1
	slog.Debug("channel suspendido (suspender + 1)")
	mutexMetricasPorProceso.Lock()

	metricas := MetricasPorProceso[paquete.NUMERO_PID]
	metricas.CANT_BAJADAS_A_SWAP += 1
	MetricasPorProceso[paquete.NUMERO_PID] = metricas

	mutexMetricasPorProceso.Unlock()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Proceso suspendido con exito."))
	slog.Debug(fmt.Sprintf("PID: %d - Proceso suspendido y guardado en swap", paquete.NUMERO_PID))
}

// func DebugSwapCompleto() {

// 	rutaSwap := filepath.Join(RutaModulo, ClientConfig.SWAPFILE_PATH)
// 	swapfile, err := os.OpenFile(rutaSwap, os.O_RDONLY, 0644)
// 	if err != nil {
// 		slog.Error(fmt.Sprintf("Error abriendo swap para debug: %v", err))
// 		return
// 	}
// 	defer swapfile.Close()

// 	procesos, err := LeerProcesosSwap(swapfile)

// 	if err != nil {
// 		slog.Error("No se pueden recuperar los procesos de swap")
// 		return
// 	}
// 	slog.Debug("== DEBUG CONTENIDO ACTUAL DE SWAP ==")

// 	for i, proceso := range procesos {
// 		/* 		if err != nil {
// 			if errors.Is(err, io.EOF) {
// 				break
// 			}
// 			slog.Error(fmt.Sprintf("Error decodificando proceso #%d: %v", i, err))
// 			break
// 		} */
// 		slog.Debug(fmt.Sprintf("Proceso #%d - PID: %d - Data: %v", i, proceso.PID, proceso.Data))
// 	}
// 	slog.Debug("== FIN DEBUG SWAP ==")
// }

func DesSuspenderProceso(w http.ResponseWriter, r *http.Request) {
	paquete := globales.PID{}
	paquete = globales.DecodificarPaquete(w, r, &paquete)

	slog.Debug(fmt.Sprintf("Proceso a deswapear: %d", paquete.NUMERO_PID))

	procesoMemoria, errProceso := ObtenerProceso(paquete.NUMERO_PID)
	<-procesoMemoria.Suspendido
	slog.Debug("channel suspendido (desuspender - 1)")

	if errProceso != nil {
		procesoMemoria.Suspendido <- 1
		slog.Debug(fmt.Sprintf("No se encontro el proceso en la memoria. PID %d: %v", paquete.NUMERO_PID, errProceso))
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("No se encontro el proceso en la memoria."))
		return
	}

	procesoObjetivo, err := buscarProcesoEnSwap(paquete.NUMERO_PID)
	if err != nil {
		procesoMemoria.Suspendido <- 1
		slog.Debug(fmt.Sprintf("No se pudo encontrar el proceso de pid %d en swap: %v",paquete.NUMERO_PID, err))
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error leyendo el archivo de swap."))
		return
	}

	asignado := ReservarMemoria(len(procesoObjetivo.Data), procesoMemoria.TablaPaginas)
    if !asignado {
		procesoMemoria.Suspendido <- 1
		slog.Info("No se pudo asignar la memoria solicitada al proceso a desuspender")
        w.WriteHeader(http.StatusInsufficientStorage)
        w.Write([]byte("No se pudo asignar la memoria solicitada."))
        return
    }

	EscribirTablaPaginas(procesoMemoria, procesoObjetivo.Data)
	slog.Debug("Despues de escribir tabla de paginas")
	mutexMetricasPorProceso.Lock()
	metricas := MetricasPorProceso[paquete.NUMERO_PID]
	metricas.CANT_SUBIDAS_A_MEMORIA += 1
	MetricasPorProceso[paquete.NUMERO_PID] = metricas
	mutexMetricasPorProceso.Unlock()
	procesoMemoria.Suspendido <- 1

	go borrarEntradaDeSwap(*procesoMemoria)
	slog.Info(fmt.Sprintf("## PID: %d - Proceso desuspendido y cargado en memoria", paquete.NUMERO_PID))
	slog.Debug("channel suspendido (desuspender + 1)")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Proceso des suspendido con exito."))
}

func buscarProcesoEnSwap(pid int) (ProcesoSwap, error) {
	mutexArchivoSwap.Lock()
	defer mutexArchivoSwap.Unlock()
	rutaSwap := filepath.Join(RutaModulo, ClientConfig.SWAPFILE_PATH)
	swapfile, errApertura := os.OpenFile(rutaSwap, os.O_RDONLY, 0644)
	if errApertura != nil {
		slog.Error(fmt.Sprintf("Hubo un error abriendo el archivo de Swap: %v", errApertura))
		return ProcesoSwap{}, fmt.Errorf("no se pudo abrir el archivo")
	}
	defer swapfile.Close()

	var procesoObjetivo ProcesoSwap
	found := false
	for !found {
		var newPid int32
		err := binary.Read(swapfile, binary.LittleEndian, &newPid)
		if err == io.EOF {
			break
		}
		slog.Debug(fmt.Sprintf("PID LEIDO DE SWAP: %d \n", newPid))

		if err != nil {
			return ProcesoSwap{}, fmt.Errorf("error leyendo PID: %v", err)
		}

		var dataLen int32
		if err := binary.Read(swapfile, binary.LittleEndian, &dataLen); err != nil {
			return ProcesoSwap{}, fmt.Errorf("error leyendo tamaño: %v", err)
		}

		slog.Debug(fmt.Sprintf("LONGITUD : %d \n", dataLen))

		// Si no es el pid que estoy buscando, salteo los datos y sigo buscando en el resto del archivo
		if newPid != int32(pid) {
			swapfile.Seek(int64(dataLen), 1)
			continue
		}

		// Si llegue a este punto, es el PID que estoy buscando
		data := make([]byte, dataLen)
		_, err = io.ReadFull(swapfile, data)
		if err != nil {
			return ProcesoSwap{}, fmt.Errorf("error leyendo data: %v", err)
		}

		procesoObjetivo = ProcesoSwap{
			PID:  int(pid),
			Data: data,
		}

		found = true
	}

	if !found {
		slog.Debug("No se encontro el proceso a des suspender en Swap.")
		//mutexArchivoSwap.Unlock()
		return ProcesoSwap{}, fmt.Errorf("no se encontro el proceso solicitado")
	}

	return procesoObjetivo, nil
}

func borrarEntradaDeSwap(proceso Proceso) error {
	mutexArchivoSwap.Lock()
	defer mutexArchivoSwap.Unlock()
	rutaSwap := filepath.Join(RutaModulo, ClientConfig.SWAPFILE_PATH)
	swapfile, errApertura := os.OpenFile(rutaSwap, os.O_APPEND|os.O_RDWR, 0644)
	if errApertura != nil {
		slog.Error(fmt.Sprintf("Hubo un error abriendo el archivo de Swap. PID %d: %v", proceso.PID, errApertura))

		return fmt.Errorf("no se pudo abrir el archivo")
	}

	procesos, err := LeerProcesosSwap(swapfile)
	swapfile.Close()
	if err != nil {
		slog.Error(fmt.Sprintf("Error abriendo swap para debug: %v", err))
		return fmt.Errorf("no se pudo leer correctamente el archivo de swap")
	}

	nuevoContenido := make([]ProcesoSwap, 0)

	for _, p := range procesos {
		if p.PID == proceso.PID {
			continue
		}
		nuevoContenido = append(nuevoContenido, p)
	}

	// Reescribir archivo
	file, _ := os.Create(rutaSwap)
	defer file.Close()
	for _, p := range nuevoContenido {
		EscribirProcesoSwap(file, p)
	}

	return nil
}

func ConcatenarDatosProceso(PID int) []byte {
	procesoMemoria, err := ObtenerProceso(PID)
	if err != nil {
		panic(err)
	}
	marcosAsignados := make([]int, 0)
	ObtenerMarcosAsignados(PID, procesoMemoria.TablaPaginas, 1, &marcosAsignados)
	buffer := make([]byte, 0)
	for _, marco := range marcosAsignados {
		inicio := marco * ClientConfig.PAGE_SIZE
		fin := ClientConfig.PAGE_SIZE * (marco + 1)
		buffer = append(buffer, MemoriaDeUsuario[inicio:fin]...)
		slog.Debug(fmt.Sprintf("Marco %d: %v", marco, MemoriaDeUsuario[inicio:fin]))
	}

	return buffer
}

func ObtenerProceso(PID int) (*Proceso, error) {
	for _, proceso := range ProcesosEnMemoria {
		if proceso.PID == PID {
			return proceso, nil
		}
	}
	return nil, fmt.Errorf("Proceso con PID %d no encontrado en memoria", PID)
}
