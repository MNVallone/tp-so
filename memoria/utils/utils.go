package utils

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"globales"
	"globales/servidor"
	"log"
	"log/slog"
	"math"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

// --------- VARIABLES DE MEMORIA --------- //
var ClientConfig *Config

// var instruccionesProcesos map[int][]string // mapa de instrucciones por PID
var instruccionesProcesos = make(map[int]map[int]string)

var mutexInstrucciones sync.Mutex // Mutex para proteger el acceso al mapa de instrucciones

// var tablasPorProceso[pid] = make(map[int]*NodoTablaPaginas)

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
	LOG_niveles      string `json:"log_niveles"`
	DUMP_PATH        string `json:"dump_path"`
	SCRIPTS_PATH     string `json:"scripts_path"`
}

// Para la memoria, un proceso se reduce a su ID y su Tabla de Paginas.
type Proceso struct {
	PID          int
	TablaPaginas *NodoTablaPaginas
	Suspendido   bool
}

type ProcesoSwap struct {
	PID  int
	Data []byte
}

type EspacioMemoriaPeticion struct {
	TamanioSolicitado int `json:"tamanio_solicitado"`
}

type EspacioMemoriaRespuesta struct {
	EspacioDisponible int  `json:"espacio_disponible"`
	Exito             bool `json:"exito"`
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
	err := os.WriteFile(ClientConfig.SWAPFILE_PATH, []byte{}, 0644)
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

	return config
}

func delayDeMemoria() {
	time.Sleep(time.Duration(ClientConfig.MEMORY_DELAY) * time.Millisecond) // Simula el delay de acceso a memoria
}

func delayDeSwap() {
	time.Sleep(time.Duration(ClientConfig.SWAP_DELAY) * time.Millisecond) // Simula el delay de acceso a swap
}

func LeerArchivoDePseudocodigo(rutaArchivo string, pid int) {
	file, err := os.Open(rutaArchivo)
	if err != nil {
		// Si hay error al abrir (ej: no existe), termina el programa
		log.Fatalf("Error al abrir el archivo '%s': %v", rutaArchivo, err)
	}
	// 2. Asegurar que el archivo se cierre al final de la función main
	// Es importante liberar los recursos.
	defer file.Close()

	// 4. Crear un scanner para leer el archivo línea por línea
	scanner := bufio.NewScanner(file)

	// 5. Inicializar contador para el número de línea
	lineNumber := 0

	// 6. Inicializar el mapa de instrucciones para el proceso
	mutexInstrucciones.Lock()
	if instruccionesProcesos[pid] == nil {
		instruccionesProcesos[pid] = make(map[int]string)
	}
	mutexInstrucciones.Unlock()

	// 6. Leer el archivo línea por línea
	fmt.Printf("Leyendo el archivo '%s'...\n", rutaArchivo)
	for scanner.Scan() {
		// Obtener el texto de la línea actual
		lineText := scanner.Text()
		// Guardar en el mapa: clave = número de línea, valor = texto de la línea

		mutexInstrucciones.Lock()
		instruccionesProcesos[pid][lineNumber] = lineText
		mutexInstrucciones.Unlock()

		// Incrementar el número de línea (empezamos en 0)
		lineNumber++
	}

	// 7. Verificar si hubo errores durante el escaneo (distintos a EOF)
	if err := scanner.Err(); err != nil {
		log.Fatalf("Error al leer el archivo '%s': %v", rutaArchivo, err)
	}

	// 8. ¡Listo! El mapa 'linesMap' ahora contiene las líneas.
	fmt.Printf("Archivo leído correctamente. Se encontraron %d líneas.\n", len(instruccionesProcesos[pid]))

	// Opcional: Imprimir el contenido del mapa
	fmt.Println("Contenido del mapa:")
	for num, linea := range instruccionesProcesos[pid] {
		fmt.Printf("Línea %d: %s\n", num, linea)
	}
}

// --------- HANDLERS DEL CPU --------- //
func AtenderCPU(w http.ResponseWriter, r *http.Request) {
	var paquete servidor.PCB = servidor.RecibirPaquetesCpu(w, r)
	slog.Info("Recibido paquete CPU")
	log.Printf("%+v\n", paquete)

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
	paquete = servidor.DecodificarPaquete(w, r, &paquete)

	delayDeMemoria()

	// pid := paquete.PID
	pidString := strconv.Itoa(paquete.PID)
	pcString := strconv.Itoa(paquete.PC)

	// ya tienen que estar cargados los archivos pseucodocodigo en memoria
	// Buscar instruccion
	slog.Info(fmt.Sprintf("Buscando instrucción en memoria para PC: %s", pcString))

	instruccion, err := json.Marshal(instruccionesProcesos[paquete.PID][paquete.PC])
	if err != nil {
		http.Error(w, "Error al codificar los datos como JSON", http.StatusInternalServerError)
		return
	}

	slog.Info(fmt.Sprintf("## PID %s - Obtener Instruccion: %s - Instruccion: %s", pidString, pcString, instruccion)) // log obligatorio TODO: agregar argumentos de la instruccion

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
	paquete = servidor.DecodificarPaquete(w, r, &paquete)

	delayDeMemoria()
	respuesta := make([]byte, paquete.TAMANIO)

	//TODO ver como afecta a las metricas de memoria
	// Si me llega un byte que es multiplo del tamaño de la pagina, leo la pagina completa

	mutexMemoria.Lock()
	for i := 0; i < paquete.TAMANIO; i++ {
		respuesta[i] = MemoriaDeUsuario[paquete.DIRECCION+i]
	}
	mutexMemoria.Unlock()

	slog.Info(fmt.Sprintf("## PID: %d - Lectura - Dir.Física: %d - Tamaño: %v", paquete.PID, paquete.DIRECCION, paquete.TAMANIO))

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
	paquete = servidor.DecodificarPaquete(w, r, &paquete)
	delayDeMemoria()

	informacion := []byte(paquete.DATOS)

	//TODO ver como afecta a las metricas de memoria

	mutexMemoria.Lock()
	for i := 0; i < len(informacion); i++ {
		MemoriaDeUsuario[paquete.DIRECCION+i] = informacion[i]
	}
	mutexMemoria.Unlock()

	slog.Info(fmt.Sprintf("## PID: %d - Escritura - Dir.Física: %d - Tamaño: %v", paquete.PID, paquete.DIRECCION, len(paquete.DATOS)))

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
	paquete = servidor.DecodificarPaquete(w, r, &paquete)

	slog.Info(fmt.Sprintf("## PID: %d - Memory Dump solicitado", paquete.NUMERO_PID))

	delayDeMemoria()

	buffer := new(bytes.Buffer)
	encoder := gob.NewEncoder(buffer)

	datosProceso := ConcatenarDatosProceso(paquete.NUMERO_PID)

	encoder.Encode(datosProceso)
	data := buffer.Bytes()
	slog.Debug(fmt.Sprintf("Buffer bytes: %v", data))

	nombreArchivo := fmt.Sprintf("%s/%d-%d.dmp", ClientConfig.DUMP_PATH, paquete.NUMERO_PID, time.Now().Unix())

	file, err := os.OpenFile(nombreArchivo, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		panic(err)
	}

	_, errWrite := file.Write(data) // data es []byte
	if errWrite != nil {
		panic(errWrite)
	}

	file.Close()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// --------- HANDLERS DEL KERNEL --------- //
func InicializarProceso(w http.ResponseWriter, r *http.Request) {
	var peticion globales.MEMORIA_CREACION_PROCESO
	peticion = servidor.DecodificarPaquete(w, r, &peticion)

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
		Suspendido:   false,
	}

	mutexProcesosEnMemoria.Lock()
	ProcesosEnMemoria = append(ProcesosEnMemoria, &nuevoProceso)
	mutexProcesosEnMemoria.Unlock()

	mutexMetricasPorProceso.Lock()
	MetricasPorProceso[peticion.PID] = METRICAS_PROCESO{}
	mutexMetricasPorProceso.Unlock()

	// 4. Cargar el archivo de pseudocodigo
	LeerArchivoDePseudocodigo(peticion.RutaArchivoPseudocodigo, peticion.PID)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func FinalizarProceso(w http.ResponseWriter, r *http.Request) {
	paquete := globales.DestruirProceso{}
	paquete = servidor.DecodificarPaquete(w, r, &paquete)

	found := false

	delayDeMemoria()

	slog.Debug(fmt.Sprintf("Procesos en memoria al inicio de la funcion: %v", ProcesosEnMemoria))

	for i, p := range ProcesosEnMemoria {
		if p.PID == paquete.PID {
			found = true
			slog.Debug("Proceso encontrado en ProcesosEnMemoria")
			// Desasignar marcos de memoria
			DesasignarMarcos(p.TablaPaginas, 1)
			ProcesosEnMemoria = remove(ProcesosEnMemoria, i)
			slog.Debug(fmt.Sprintf("Proceso con PID %d destruido exitosamente.", paquete.PID))
			break
		}
	}

	if !found {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("No se encontro el proceso solicitado."))
		return
	}

	MostrarMetricasProceso(paquete.PID)

	mutexMetricasPorProceso.Lock()
	delete(MetricasPorProceso, paquete.PID)
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
	paquete = servidor.DecodificarPaquete(w, r, &paquete)

	// delayDeMemoria(): el delay se hace en ObtenerMarcoDeTDP

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

	slog.Info(fmt.Sprintf("PID: %d - Marco obtenido: %d", paquete.PID, marco))
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
	if semilla < numNiveles { // si es una taba intermedia
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
	maximoTamPorProceso := int(float64(ClientConfig.PAGE_SIZE) * math.Pow(float64(ClientConfig.ENTRIES_PER_PAGE), float64(ClientConfig.NUMBER_OF_LEVELS)))

	fmt.Printf("Reservando memoria. Proceso de tamanio %d solicita %d paginas. Tamanio de pag es %d\n", tamanioProceso, cant_paginas_proceso, ClientConfig.PAGE_SIZE)

	mutexMemoria.Lock()

	if len(MarcosLibres) < cant_paginas_proceso || tamanioProceso > maximoTamPorProceso {
		mutexMemoria.Unlock()
		slog.Error("No hay suficientes paginas para almacenar el proceso completo en memoria")
		return false
	}

	fmt.Printf("Asignando memoria. Marcos libres: %d. Paginas solicitadas: %d \n", len(MarcosLibres), cant_paginas_proceso)

	AsignarMarcos(TablaPaginas, 1, &cant_paginas_proceso)

	mutexMemoria.Unlock()

	return true
}

// Asigna marcos libres a las hojas que no estén ocupadas
func AsignarMarcos(node *NodoTablaPaginas, level int, marcosRestantes *int) {
	if *marcosRestantes > 0 { // ¿Quedan marcos por cargar?
		if level == ClientConfig.NUMBER_OF_LEVELS { //TODO: Queremos que nuestro último nivel sea la tabla de páginas que apunta a los marcos de memoria.

			for i := range node.Marcos {
				node.Marcos[i] = &MarcosLibres[0]
				slog.Debug(fmt.Sprintf("\n Asignando marco %d a la entrada %d del nivel %d, valor puntero: %v", *node.Marcos[i], i, level, node.Marcos[i]))
				MarcosLibres = MarcosLibres[1:]
				slog.Debug(fmt.Sprintf("\n Longitud de marcos libres %d", len(MarcosLibres)))
				nuevosMarcos := *marcosRestantes - 1
				*marcosRestantes = nuevosMarcos
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
	if level == ClientConfig.NUMBER_OF_LEVELS { //TODO: Queremos que nuestro último nivel sea la tabla de páginas que apunta a los marcos de memoria.
		for i := range node.Marcos {
			if node.Marcos[i] == nil {
				break
			}
			slog.Debug(fmt.Sprintf("\n Numero de marco: %d", *node.Marcos[i]))
			MarcosLibres = append(MarcosLibres, *node.Marcos[i]) // Agrega el marco liberado
			node.Marcos[i] = nil                                 // Limpia la referencia al marco
			slog.Debug(fmt.Sprintf("\n Longitud de marcos libres %d", len(MarcosLibres)))
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

// --------- PARA TESTEAR --------- //

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
				break
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

func ObtenerMarcoEnTabla(raiz *NodoTablaPaginas, indices []int) *NodoTablaPaginas {
	nodo := raiz
	for _, idx := range indices {
		nodo = nodo.Children[idx]
	}
	return nodo
}

func LeerPaginaCompleta(w http.ResponseWriter, r *http.Request) {
	paquete := globales.LeerMarcoMemoria{}
	paquete = servidor.DecodificarPaquete(w, r, &paquete)

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
	paquete = servidor.DecodificarPaquete(w, r, &paquete)

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

func DesSuspenderProceso(w http.ResponseWriter, r *http.Request) {
	paquete := globales.PID{}
	paquete = servidor.DecodificarPaquete(w, r, &paquete)

	procesoMemoria, err := ObtenerProceso(paquete.NUMERO_PID)

	if err != nil {
		slog.Error(fmt.Sprintf("No se encontro el proceso en la memoria. PID %d: %v", paquete.NUMERO_PID, err))
	}

	var deserializedStructs []ProcesoSwap
	mutexArchivoSwap.Lock()
	archivo, _ := os.ReadFile(ClientConfig.SWAPFILE_PATH)

	buffer := bytes.NewBuffer(archivo)
	decoder := gob.NewDecoder(buffer)

	var procesoObjetivo ProcesoSwap
	for {
		var s ProcesoSwap
		err := decoder.Decode(&s)
		if err != nil {
			break
		}
		// Si es el proceso que busco, ya me lo quedo pues no voy a escribirlo en el archivo de Swap.
		if s.PID == procesoMemoria.PID {
			procesoObjetivo = s
		} else {
			deserializedStructs = append(deserializedStructs, s)
		}
	}

	// Escribo el archivo de Swap quitando el proceso que Des suspendi.
	buffer = new(bytes.Buffer)
	encoder := gob.NewEncoder(buffer)
	for _, s := range deserializedStructs {
		encoder.Encode(s)
	}
	data := buffer.Bytes()
	os.WriteFile(ClientConfig.SWAPFILE_PATH, data, 0644)

	mutexArchivoSwap.Unlock()

	ReservarMemoria(len(procesoObjetivo.Data), procesoMemoria.TablaPaginas)
	EscribirTablaPaginas(procesoMemoria, procesoObjetivo.Data)

	w.WriteHeader(http.StatusOK)
}

// Toma la tabla de paginas de un proceso y escribe todos los datos en los marcos asignados, sobreescribiendo la informacion previa.
func EscribirTablaPaginas(procesoMemoria *Proceso, datos []byte) bool {
	var marcosEscritura []int
	ObtenerMarcosAsignados(procesoMemoria.PID, procesoMemoria.TablaPaginas, 0, &marcosEscritura)

	mutexMemoria.Lock()
	contador := 0
	for _, marco := range marcosEscritura {
		datosEscritura := datos[contador : contador+ClientConfig.PAGE_SIZE]
		copy(MemoriaDeUsuario[marco:], datosEscritura)
		contador += ClientConfig.PAGE_SIZE
	}
	mutexMemoria.Unlock()

	return true
}

func SuspenderProceso(w http.ResponseWriter, r *http.Request) {
	paquete := globales.PID{}
	paquete = servidor.DecodificarPaquete(w, r, &paquete)

	procesoMemoria, err := ObtenerProceso(paquete.NUMERO_PID)

	if err != nil {
		slog.Error(fmt.Sprintf("No se encontro el proceso en la memoria. PID %d: %v", paquete.NUMERO_PID, err))
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("No se encontro el proceso en la memoria."))
		return
	}

	delayDeSwap()

	buffer := new(bytes.Buffer) // Buffer de bytes
	encoder := gob.NewEncoder(buffer)

	datosProceso := ConcatenarDatosProceso(paquete.NUMERO_PID)
	procesoASuspeder := ProcesoSwap{
		PID:  paquete.NUMERO_PID,
		Data: datosProceso,
	}

	encoder.Encode(procesoASuspeder)
	data := buffer.Bytes()
	slog.Debug(fmt.Sprintf("Buffer bytes: %v", data))

	slog.Info("Proceso concatenado y codificado.")

	mutexArchivoSwap.Lock()

	file, err := os.OpenFile(ClientConfig.SWAPFILE_PATH, os.O_APPEND|os.O_RDWR, os.ModeAppend)
	if err != nil {
		slog.Error(fmt.Sprintf("Hubo un error con el archivo de swap: %v", err))
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Hubo un error con el archivo de swap."))
		mutexArchivoSwap.Unlock()
		return
	}

	_, errWrite := file.Write(data) // data es []byte
	if errWrite != nil {
		slog.Error(fmt.Sprintf("Hubo un error escribiendo el archivo de swap: %v", err))
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Hubo un error escribiendo el archivo de swap."))
		mutexArchivoSwap.Unlock()
		return
	}

	file.Close()
	procesoMemoria.Suspendido = true
	mutexArchivoSwap.Unlock()
	slog.Info("Archivo de swap escrito.")

	DesasignarMarcos(procesoMemoria.TablaPaginas, 1)

	mutexMetricasPorProceso.Lock()

	metricas := MetricasPorProceso[paquete.NUMERO_PID]
	metricas.CANT_BAJADAS_A_SWAP += 1
	MetricasPorProceso[paquete.NUMERO_PID] = metricas

	mutexMetricasPorProceso.Unlock()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Proceso suspendido con exito."))
	slog.Info(fmt.Sprintf("PID: %d - Proceso suspendido y guardado en swap", paquete.NUMERO_PID))
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
