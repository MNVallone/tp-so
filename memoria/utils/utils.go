package utils

import (
	"bufio"
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
)

// --------- VARIABLES DE MEMORIA --------- //
var ClientConfig *Config

// var instruccionesProcesos map[int][]string // mapa de instrucciones por PID
var instruccionesProcesos = make(map[int]map[int]string)

var Listado_Metricas []METRICAS_PROCESO //Cuando se reserva espacio en memoria lo agregamos aca
// var mutexMetricas sync.Mutex

var MemoriaDeUsuario []byte // Simulacion de la memoria de usuario
var MarcosLibres []int
var mutexMemoria sync.Mutex // Mutex para proteger el acceso a la memoria de usuario

var ProcesosEnMemoria []*Proceso

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
}

type EspacioMemoriaPeticion struct {
	TamanioSolicitado int `json:"tamanio_solicitado"`
}

type EspacioMemoriaRespuesta struct {
	EspacioDisponible int  `json:"espacio_disponible"`
	Exito             bool `json:"exito"`
}

type METRICAS_PROCESO struct { //Cuando se reserva espacio en memoria inicializamos esta estructura
	PID                            int `json:"pid"`
	CANT_ACCESOS_TABLA_DE_PAGINAS  int `json:"cant_accesos_tabla_de_paginas"`
	CANT_INSTRUCCIONES_SOLICITADAS int `json:"cant_instrucciones_solicitadas"`
	CANT_BAJADAS_A_SWAP            int `json:"cant_accesos_swap"`
	CANT_SUBIDAS_A_MEMORIA         int `json:"cant_subidas_a_memoria"`
	CANT_LECTURAS_MEMORIA          int `json:"cant_lecturas_memoria"`
	CANT_ESCRITURAS_MEMORIA        int `json:"cant_escrituras_memoria"`
}
type NodoTablaPaginas struct {
	Children []*NodoTablaPaginas // Para niveles intermedios
	Frame    int                 // Solo para el último nivel
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
	if instruccionesProcesos[pid] == nil {
		instruccionesProcesos[pid] = make(map[int]string)
	}

	// 6. Leer el archivo línea por línea
	fmt.Printf("Leyendo el archivo '%s'...\n", rutaArchivo)
	for scanner.Scan() {
		// Obtener el texto de la línea actual
		lineText := scanner.Text()
		// Guardar en el mapa: clave = número de línea, valor = texto de la línea
		instruccionesProcesos[pid][lineNumber] = lineText

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

func ReservarMemoria(tamanio int, TablaPaginas *NodoTablaPaginas) bool {
	div := float64(tamanio) / float64(ClientConfig.PAGE_SIZE)
	cant_paginas := int(math.Ceil(float64(div)))
	fmt.Printf("Reservando memoria. Proceso de tamanio %d solicita %d paginas. Tamanio de pag es %d\n", tamanio, cant_paginas, ClientConfig.PAGE_SIZE)

	// Region critica. Asignacion de memoria.
	mutexMemoria.Lock()

	if len(MarcosLibres) < cant_paginas {
		slog.Error("Error al solicitar memoria. No hay marcos disponibles.")
		return false
	}

	fmt.Printf("Asignando memoria. Marcos libres: %d. Paginas solicitadas: %d \n", len(MarcosLibres), cant_paginas)

	AsignarMarcos(TablaPaginas, 0, &cant_paginas)

	mutexMemoria.Unlock()
	// Fin de la region critica de asignacion de memoria.

	return true
}

func InicializarMemoria() {
	// Creo la memoria de usuario
	MemoriaDeUsuario = make([]byte, ClientConfig.MEMORY_SIZE)
	// Divido la memoria en marcos
	var cant_paginas int = ClientConfig.MEMORY_SIZE / ClientConfig.PAGE_SIZE
	MarcosLibres = make([]int, cant_paginas)
	for idx := range cant_paginas {
		MarcosLibres[idx] = idx
	}
}

func remove(s []*Proceso, i int) []*Proceso {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}

// --------- HANDLERS DEL SERVIDOR --------- //

func AtenderCPU(w http.ResponseWriter, r *http.Request) {
	var paquete servidor.PCB = servidor.RecibirPaquetesCpu(w, r)
	slog.Info("Recibido paquete CPU")
	log.Printf("%+v\n", paquete)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func CrearProceso(w http.ResponseWriter, r *http.Request) {
	var peticion globales.MEMORIA_CREACION_PROCESO
	// Sacar esto. Hay que abstraerlo a otra funcion
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&peticion)
	if err != nil {
		slog.Error(fmt.Sprintf("Error decodificando petición: %s", err.Error()))
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Error decodificando petición"))
		return
	}

	// 1. Creo la tabla de paginas del proceso y la guardo.
	TablaDePaginas := CrearTablaPaginas(0, ClientConfig.NUMBER_OF_LEVELS, ClientConfig.ENTRIES_PER_PAGE)

	// 2. Le asigno el espacio solicitado (si es posible)
	asignado := ReservarMemoria(peticion.Tamanio, TablaDePaginas)

	if !asignado {
		w.WriteHeader(http.StatusInsufficientStorage)
		w.Write([]byte("No se pudo asignar la memoria solicitada."))
	}

	// var nuevoProceso Proceso = Proceso{
	// 	PID:          peticion.PID,
	// 	TablaPaginas: TablaDePaginas,
	// }

	// append(ProcesosEnMemoria, &nuevoProceso)

	// 3. Cargar el archivo de pseudocodigo
	LeerArchivoDePseudocodigo(peticion.RutaArchivoPseudocodigo, peticion.PID)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func DumpearProceso(w http.ResponseWriter, r *http.Request) {
	/*

		TODO
		agregar mutex

		paquete := globales.PeticionDump{}
		paquete = servidor.DecodificarPaquete(w, r, &paquete)



		// slog.Info(fmt.Sprintf("## PID %s - Memory Dump solicitado: %s", pidString)) // log obligatorio

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))*/
}

func DevolverInstruccion(w http.ResponseWriter, r *http.Request) {
	paquete := globales.PeticionInstruccion{}
	paquete = servidor.DecodificarPaquete(w, r, &paquete)

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

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(instruccion))
}

func LeerDireccion(w http.ResponseWriter, r *http.Request) {
	paquete := globales.LeerMemoria{}
	paquete = servidor.DecodificarPaquete(w, r, &paquete)
	respuesta := make([]byte, paquete.TAMANIO)

	//TODO ver como afecta a las metricas de memoria

	mutexMemoria.Lock()
	for i := 0; i < paquete.TAMANIO; i++ {
		respuesta[i] = MemoriaDeUsuario[paquete.DIRECCION+i]
	}
	mutexMemoria.Unlock()

	w.WriteHeader(http.StatusOK)
	w.Write(respuesta)
}

func EscribirDireccion(w http.ResponseWriter, r *http.Request) {
	paquete := globales.EscribirMemoria{}
	paquete = servidor.DecodificarPaquete(w, r, &paquete)
	informacion := []byte(paquete.DATOS)

	//TODO ver como afecta a las metricas de memoria

	mutexMemoria.Lock()
	for i := 0; i < len(informacion); i++ {
		MemoriaDeUsuario[paquete.DIRECCION+i] = informacion[i]
	}
	mutexMemoria.Unlock()
	w.WriteHeader(http.StatusOK)
}

func DestruirProceso(w http.ResponseWriter, r *http.Request) {
	paquete := globales.DestruirProceso{}
	paquete = servidor.DecodificarPaquete(w, r, &paquete)
	found := false
	for proc := range len(ProcesosEnMemoria) {
		if ProcesosEnMemoria[proc].PID == paquete.PID {
			found = true
			remove(ProcesosEnMemoria, proc)
		}
	}

	if !found {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("No se encontro el proceso solicitado."))
		return
	}

	slog.Info("PID: %d - Proceso Destruido - Metricas - [TBD]")

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Proceso eliminado con exito."))
}

// --------- PAGINACION MULTINIVEL --------- //

func CrearTablaPaginas(semilla, numNiveles, pagsPorNivel int) *NodoTablaPaginas {
	nodo := &NodoTablaPaginas{}
	if semilla < numNiveles-1 {
		nodo.Children = make([]*NodoTablaPaginas, pagsPorNivel)
		for i := 0; i < pagsPorNivel; i++ {
			nodo.Children[i] = CrearTablaPaginas(semilla+1, numNiveles, pagsPorNivel)
		}
	} else {
		// Último nivel: inicializar Frame a nil o a un valor por defecto
		nodo.Frame = -1
	}
	return nodo
}

func ObtenerMarcoEnTabla(raiz *NodoTablaPaginas, indices []int) *NodoTablaPaginas {
	nodo := raiz
	for _, idx := range indices {
		nodo = nodo.Children[idx]
	}
	return nodo
}

// Asigna marcos libres a las hojas que no estén ocupadas
func AsignarMarcos(node *NodoTablaPaginas, level int, marcosRestantes *int) {
	if *marcosRestantes > 0 {
		if level == ClientConfig.NUMBER_OF_LEVELS-1 {
			if node.Frame == -1 { // SOLO si la página está libre
				node.Frame = MarcosLibres[0]
				MarcosLibres = MarcosLibres[1:] // Quita el marco asignado
				nuevosMarcos := *marcosRestantes - 1
				*marcosRestantes = nuevosMarcos
				fmt.Printf("Asignada la pagina %d, marcos restantes: %d \n", node.Frame, *marcosRestantes)

			}
		} else {
			for i := 0; i < ClientConfig.ENTRIES_PER_PAGE; i++ {
				AsignarMarcos(node.Children[i], level+1, marcosRestantes)
			}
		}
	}
}

// Asigna marcos libres a las hojas que no estén ocupadas
func ObtenerMarcosAsignados(node *NodoTablaPaginas, level int, marcosAsignados *[]int) {
	if level == ClientConfig.NUMBER_OF_LEVELS-1 {
		if node.Frame != -1 { // SOLO si la página está libre
			*marcosAsignados = append(*marcosAsignados, node.Frame)
		}
	} else {
		for i := 0; i < ClientConfig.ENTRIES_PER_PAGE; i++ {
			ObtenerMarcosAsignados(node.Children[i], level+1, marcosAsignados)
		}
	}
}
