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
var EspacioUsado int = 0

// var instruccionesProcesos map[int][]string // mapa de instrucciones por PID
var instruccionesProcesos = make(map[int]map[int]string)

var Listado_Metricas []METRICAS_PROCESO //Cuando se reserva espacio en memoria lo agregamos aca
// var mutexMetricas sync.Mutex

var MemoriaDeUsuario []byte // Simulacion de la memoria de usuario
var MarcosLibres []int
var mutexMemoria sync.Mutex // Mutex para proteger el acceso a la memoria de usuario

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
	Frame    int                 // Solo para el último nivel (hoja)
}

// --------- FUNCIONES DE MEMORIA --------- //
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

func AtenderCPU(w http.ResponseWriter, r *http.Request) {
	var paquete servidor.PCB = servidor.RecibirPaquetesCpu(w, r)
	slog.Info("Recibido paquete CPU")
	log.Printf("%+v\n", paquete)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func VerificarEspacioDisponible(w http.ResponseWriter, r *http.Request) {
	var peticion EspacioMemoriaPeticion

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&peticion)
	if err != nil {
		slog.Error(fmt.Sprintf("Error decodificando petición: %s", err.Error()))
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Error decodificando petición"))
		return
	}

	slog.Info(fmt.Sprintf("Solicitado espacio en memoria: %d bytes", peticion.TamanioSolicitado))

	espacioDisponible := ClientConfig.MEMORY_SIZE - EspacioUsado

	respuesta := EspacioMemoriaRespuesta{
		EspacioDisponible: espacioDisponible,
		Exito:             peticion.TamanioSolicitado <= espacioDisponible,
	}

	jsonResp, err := json.Marshal(respuesta)
	if err != nil {
		slog.Error(fmt.Sprintf("Error codificando respuesta: %s", err.Error()))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonResp)
}

func ReservarEspacio(w http.ResponseWriter, r *http.Request) {

	var peticion EspacioMemoriaPeticion

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&peticion)
	if err != nil {
		slog.Error(fmt.Sprintf("Error decodificando petición: %s", err.Error()))
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Error decodificando petición"))
		return
	}
	slog.Info(fmt.Sprintf("Reservando espacio en memoria: %d bytes", peticion.TamanioSolicitado))

	espacioDisponible := ClientConfig.MEMORY_SIZE - EspacioUsado
	exito := peticion.TamanioSolicitado <= espacioDisponible

	respuesta := EspacioMemoriaRespuesta{
		EspacioDisponible: espacioDisponible,
		Exito:             exito,
	}

	if exito {
		EspacioUsado += peticion.TamanioSolicitado
		slog.Info(fmt.Sprintf("Espacio reservado. Espacio usado total: %d/%d", EspacioUsado, ClientConfig.MEMORY_SIZE))
	} else {
		slog.Info(fmt.Sprintf("No hay suficiente espacio para reservar: %d > %d", peticion.TamanioSolicitado, espacioDisponible))
	}

	jsonResp, err := json.Marshal(respuesta)
	if err != nil {
		slog.Error(fmt.Sprintf("Error codificando respuesta: %s", err.Error()))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonResp)
}

func LiberarEspacio(w http.ResponseWriter, r *http.Request) {
	var peticion globales.PIDAEliminar
	peticion = servidor.DecodificarPaquete(w, r, &peticion)

	pid := peticion.NUMERO_PID
	tamanioAEliminar := peticion.TAMANIO

	slog.Info(fmt.Sprintf("Liberando espacio en memoria: %d bytes", tamanioAEliminar))

	if tamanioAEliminar > EspacioUsado {
		EspacioUsado = 0
		slog.Warn("Se intentó liberar más espacio del usado. Espacio usado reseteado a 0")
	} else {
		EspacioUsado -= tamanioAEliminar
		delete(instruccionesProcesos, pid) // elimina clave (PID) del mapa
	}

	respuesta := EspacioMemoriaRespuesta{
		EspacioDisponible: ClientConfig.MEMORY_SIZE - EspacioUsado,
		Exito:             true,
	}

	slog.Info(fmt.Sprintf("Espacio liberado. Espacio usado total: %d/%d", EspacioUsado, ClientConfig.MEMORY_SIZE))

	jsonResp, err := json.Marshal(respuesta)
	if err != nil {
		slog.Error(fmt.Sprintf("Error codificando respuesta: %s", err.Error()))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonResp)
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
				if len(MarcosLibres) == 0 {
					panic("No hay suficientes marcos libres para todas las páginas libres")
				}
				node.Frame = MarcosLibres[0]
				MarcosLibres = MarcosLibres[1:] // Quita el marco asignado
				nuevosMarcos := *marcosRestantes - 1
				*marcosRestantes = nuevosMarcos
				fmt.Printf("Asignada la pagina %d, marcos restantes: %d \n", node.Frame, *marcosRestantes)

			}
		} else {
			slog.Info("Accediendo a tabla de transicion...")
			for i := 0; i < ClientConfig.ENTRIES_PER_PAGE; i++ {
				AsignarMarcos(node.Children[i], level+1, marcosRestantes)
			}
		}
	}
}

// Si ejecuto esta funcion es porque hay marcos libres, no necesito checkearlo.
func ReservarMemoria(tamanio int, TablaPaginas *NodoTablaPaginas) bool {
	div := float64(tamanio) / float64(ClientConfig.PAGE_SIZE)
	cant_paginas := int(math.Ceil(float64(div)))
	fmt.Printf("Reservando memoria. Proceso de tamanio %d solicita %d paginas. Tamanio de pag es %d", tamanio, cant_paginas, ClientConfig.PAGE_SIZE)

	if len(MarcosLibres) < cant_paginas {
		slog.Error("Error al solicitar memoria. No hay marcos disponibles")
		return false
	}

	// for acumulador := 0; acumulador < cant_paginas; acumulador++ {
	// 	marco_asignado := MarcosLibres[0]
	// 	MarcosLibres = MarcosLibres[1:]
	// 	marco := ObtenerMarcoEnTabla(TablaPaginas)
	// }

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
		slog.Info("Creado nodo de ultimo nivel")
	}
	return nodo
}

func CargarProcesoAMemoria(w http.ResponseWriter, r *http.Request) {
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

	// 1. Comprobar si hay marcos libres
	if len(MarcosLibres) > 0 {
		fmt.Printf("No hay marcos libres para crear el proceso con pid %d", peticion.PID)
		return
	}

	// ReservarEspacio() - Cambiar cuando haya una implementacion del manejo de la memoria para los procesos.
	// 2. Cargar el archivo de pseudocodigo
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
