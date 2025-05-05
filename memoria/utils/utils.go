package utils

import (
	"bufio"
	"encoding/json"
	"fmt"
	"globales"
	"globales/servidor"
	"log"
	"log/slog"
	"net/http"
	"os"

	// "sync"
	"strconv"
)

// --------- VARIABLES DE MEMORIA --------- //
var ClientConfig *Config
var EspacioUsado int = 0

// var instruccionesProcesos map[int][]string // mapa de instrucciones por PID
var instruccionesProcesos = make(map[int]map[int]string)

var Listado_Metricas []METRICAS_PROCESO //Cuando se reserva espacio en memoria lo agregamos aca
// var mutexMetricas sync.Mutex

var MemoriaDeUsuario []byte // Simulacion de la memoria de usuario

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
	var peticion EspacioMemoriaPeticion

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&peticion)
	if err != nil {
		slog.Error(fmt.Sprintf("Error decodificando petición: %s", err.Error()))
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Error decodificando petición"))
		return
	}
	slog.Info(fmt.Sprintf("Liberando espacio en memoria: %d bytes", peticion.TamanioSolicitado))

	if peticion.TamanioSolicitado > EspacioUsado {
		EspacioUsado = 0
		slog.Warn("Se intentó liberar más espacio del usado. Espacio usado reseteado a 0")
	} else {
		EspacioUsado -= peticion.TamanioSolicitado
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
		// Incrementar el número de línea (empezamos en 1)
		lineNumber++
		// Obtener el texto de la línea actual
		lineText := scanner.Text()
		// Guardar en el mapa: clave = número de línea, valor = texto de la línea
		instruccionesProcesos[pid][lineNumber] = lineText
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

func CargarProcesoAMemoria(w http.ResponseWriter, r *http.Request) {
	var peticion globales.MEMORIA_CREACION_PROCESO

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&peticion)
	if err != nil {
		slog.Error(fmt.Sprintf("Error decodificando petición: %s", err.Error()))
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Error decodificando petición"))
		return
	}

	// 1. Asignar memoria
	espacioDisponible := ClientConfig.MEMORY_SIZE - EspacioUsado - peticion.Tamanio
	if espacioDisponible < 0 {
		fmt.Printf("No hay espacio disponible para crear el proceso con pid %d", espacioDisponible)
		return
	}

	// ReservarEspacio() - Cambiar cuando haya una implementacion del manejo de la memoria para los procesos.
	EspacioUsado += peticion.Tamanio
	// 2. Cargar el archivo de pseudocodigo
	LeerArchivoDePseudocodigo(peticion.RutaArchivoPseudocodigo, peticion.PID)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
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

	//TODO: agregar semaforos
	for i := 0; i < paquete.TAMANIO; i++ {
		respuesta[i] = MemoriaDeUsuario[paquete.DIRECCION+i]
	}
	fmt.Print("Respuesta en binario: ", respuesta)
	fmt.Print("Respuesta en string:", string(respuesta))

	w.WriteHeader(http.StatusOK)
	w.Write(respuesta)
}

func EscribirDireccion(w http.ResponseWriter, r *http.Request) {
	paquete := globales.EscribirMemoria{}
	paquete = servidor.DecodificarPaquete(w, r, &paquete)
	informacion := []byte(paquete.DATOS)
	//TODO: agregar semaforos
	for i := 0; i < len(informacion); i++ {
		MemoriaDeUsuario[paquete.DIRECCION+i] = informacion[i]
	}

	w.WriteHeader(http.StatusOK)
}
