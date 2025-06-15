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

var Listado_Metricas []METRICAS_PROCESO //Cuando se reserva espacio en memoria lo agregamos aca
// var mutexMetricas sync.Mutex

var MemoriaDeUsuario []byte // Simulacion de la memoria de usuario
var MarcosLibres []int

var mutexMemoria sync.Mutex // Mutex para proteger el acceso a la memoria de usuario

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
	Marcos   []*int
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
	// Si me llega un byte que es multiplo del tamaño de la pagina, leo la pagina completa

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

// --------- HANDLERS DEL KERNEL --------- //
func CrearProceso(w http.ResponseWriter, r *http.Request) {
	var peticion globales.MEMORIA_CREACION_PROCESO
	peticion = servidor.DecodificarPaquete(w, r, &peticion)

	// 1. Creo la tabla de paginas del proceso y la guardo.
	TablaDePaginas := CrearTablaPaginas(1, ClientConfig.NUMBER_OF_LEVELS, ClientConfig.ENTRIES_PER_PAGE)

	// 2. Le asigno el espacio solicitado (si es posible)
	asignado := ReservarMemoria(peticion.Tamanio, TablaDePaginas)

	if !asignado {
		w.WriteHeader(http.StatusInsufficientStorage)
		w.Write([]byte("No se pudo asignar la memoria solicitada."))
	}

	// 3. Creo el proceso y lo guardo en la lista de procesos en memoria
	nuevoProceso := Proceso{
		PID:          peticion.PID,
		TablaPaginas: TablaDePaginas,
	}

	mutexProcesosEnMemoria.Lock()
	ProcesosEnMemoria = append(ProcesosEnMemoria, &nuevoProceso)
	mutexProcesosEnMemoria.Unlock()
	// 4. Cargar el archivo de pseudocodigo
	LeerArchivoDePseudocodigo(peticion.RutaArchivoPseudocodigo, peticion.PID)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func DestruirProceso(w http.ResponseWriter, r *http.Request) {
	paquete := globales.DestruirProceso{}
	paquete = servidor.DecodificarPaquete(w, r, &paquete)
	found := false

	slog.Debug(fmt.Sprintf("Procesos en memoria al inicio de la funcion: %v", ProcesosEnMemoria))

	for i, p := range ProcesosEnMemoria {
		if p.PID == paquete.PID {
			found = true
			slog.Debug(fmt.Sprintf("Proceso encontrado en ProcesosEnMemoria"))
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

	slog.Info(fmt.Sprintf("PID: %d - Proceso Destruido - Metricas - [TBD]", paquete.PID))

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Proceso eliminado con exito."))
}

func ObtenerMarco(w http.ResponseWriter, r *http.Request) {
	paquete := globales.ObtenerMarco{}
	paquete = servidor.DecodificarPaquete(w, r, &paquete)

	// Obtener el marco de memoria correspondiente
	mutexProcesosEnMemoria.Lock()
	var marco int
	for _, proceso := range ProcesosEnMemoria {
		if proceso.PID == paquete.PID {
			marco = ObtenerMarcoDeTDP(proceso.TablaPaginas, paquete.Entradas_Nivel_X, 1)
			break
		}
	}
	mutexProcesosEnMemoria.Unlock()

	/*
		if marco == 0 {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("No se encontro el marco solicitado."))
			return
		}
	*/

	slog.Info(fmt.Sprintf("PID: %d - Marco obtenido: %d", paquete.PID, marco))
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(strconv.Itoa(marco)))
}

func remove(s []*Proceso, i int) []*Proceso {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}

/*
func removeByPID(s []*Proceso, pid int) []*Proceso {
	for i, p := range s {
		if p.PID == pid {
			s[i] = s[len(s)-1]
			return s[:len(s)-1]
		}
	}
	return s // Si no lo encuentra, devuelve el slice sin cambios
}
*/
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
	/*
		// Creacion del archivo de SWAP
		err := os.WriteFile(ClientConfig.SWAPFILE_PATH, []byte{}, 0644)
		if err != nil {
			panic(err)
		}
	*/
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
			/*if node.Frame == -1 { // SOLO si la página está libre
				node.Frame = MarcosLibres[0]
				MarcosLibres = MarcosLibres[1:] // Quita el marco asignado
				nuevosMarcos := *marcosRestantes - 1
				*marcosRestantes = nuevosMarcos
				fmt.Printf("Asignada la pagina %d, marcos restantes: %d \n", node.Frame, *marcosRestantes)
			}*/

			for i := range node.Marcos {
				node.Marcos[i] = &MarcosLibres[0]
				MarcosLibres = MarcosLibres[1:]
				slog.Debug(fmt.Sprintf("\n Longitud de marcos libres %d", len(MarcosLibres)))
				nuevosMarcos := *marcosRestantes - 1
				*marcosRestantes = nuevosMarcos
			}

		} else { // No es el último nivel
			for i := 0; i < ClientConfig.ENTRIES_PER_PAGE; i++ {
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

func ObtenerMarcoDeTDP(TDP *NodoTablaPaginas, entrada_nivel_X []int, level int) int {
	slog.Debug(fmt.Sprintf("Obteniendo marco de TDP. Nivel: %d, Entradas: %v ...", level, entrada_nivel_X))
	time.Sleep(time.Duration(ClientConfig.MEMORY_DELAY) * time.Millisecond) // Simula el delay de acceso a memoria

	if level == ClientConfig.NUMBER_OF_LEVELS {
		slog.Info(fmt.Sprintf("Accediendo a direccion: %d", *TDP.Marcos[entrada_nivel_X[ClientConfig.NUMBER_OF_LEVELS-1]]))
		numeroMarco := *TDP.Marcos[entrada_nivel_X[ClientConfig.NUMBER_OF_LEVELS-1]]
		return numeroMarco // Retorna el marco de memoria al que se accede
	} else {
		return ObtenerMarcoDeTDP(TDP.Children[entrada_nivel_X[level-1]], entrada_nivel_X, level+1) // Accede al siguiente nivel
	}
}

// --------- PARA TESTEAR --------- //
// Asigna marcos libres a las hojas que no estén ocupadas
func ObtenerMarcosAsignados(node *NodoTablaPaginas, level int, marcosAsignados *[]int) {
	if level == ClientConfig.NUMBER_OF_LEVELS {
		/*if node.Frame != -1 { // SOLO si la página está libre
			*marcosAsignados = append(*marcosAsignados, node.Frame)
		}*/

		for i := range node.Marcos {
			if node.Marcos[i] == nil {
				break
			}
			slog.Info(fmt.Sprintf("\nEntrada numero %d: %d", i, *node.Marcos[i]))
			*marcosAsignados = append(*marcosAsignados, *node.Marcos[i])
		}

	} else {
		for i := 0; i < ClientConfig.ENTRIES_PER_PAGE; i++ {
			slog.Info(fmt.Sprintf("\nAccediendo a la %dº a TDP de nivel %d", i+1, level+1))
			ObtenerMarcosAsignados(node.Children[i], level+1, marcosAsignados)
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

	direccion := paquete.DIRECCION
	desplazamiento := (direccion + ClientConfig.PAGE_SIZE)

	mutexMemoria.Lock()
	memoriaLeida := MemoriaDeUsuario[direccion:desplazamiento]
	mutexMemoria.Unlock()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(memoriaLeida))
}

func EscribirPaginaCompleta(w http.ResponseWriter, r *http.Request) {
	paquete := globales.EscribirMarcoMemoria{}
	paquete = servidor.DecodificarPaquete(w, r, &paquete)

	mutexMemoria.Lock()
	for i := 0; i < len(paquete.DATOS); i++ {
		MemoriaDeUsuario[paquete.DIRECCION+i] = paquete.DATOS[i]
	}
	mutexMemoria.Unlock()

	w.WriteHeader(http.StatusOK)
}

func SuspenderProceso(w http.ResponseWriter, r *http.Request) {
	paquete := globales.SuspenderProceso{}
	paquete = servidor.DecodificarPaquete(w, r, &paquete)

	buffer := new(bytes.Buffer) // Buffer de bytes
	encoder := gob.NewEncoder(buffer)

	datosProceso := ConcatenarDatosProceso(paquete.PID)
	procesoASuspeder := ProcesoSwap{
		PID:  paquete.PID,
		Data: datosProceso,
	}
	encoder.Encode(procesoASuspeder)
	data := buffer.Bytes()
	fmt.Println("Buffer bytes:", data)
	mutexArchivoSwap.Lock()
	file, err := os.OpenFile(ClientConfig.SWAPFILE_PATH, os.O_APPEND, 0644)
	if err != nil {
		panic(err)
	}
	//TODO verificar si el PID existe y si no, crearlo

	_, errWrite := file.Write(data) // data es []byte
	if errWrite != nil {
		panic(errWrite)
	}
	file.Close()
	mutexArchivoSwap.Unlock()

	tablaDePaginas, err := ObtenerTablaPaginas(paquete.PID)
	if err != nil {
		slog.Error(fmt.Sprintf("Error al obtener la tabla de paginas del proceso %d: %v", paquete.PID, err))
	}
	DesasignarMarcos(tablaDePaginas, 1)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Proceso suspendido con exito."))
	slog.Info(fmt.Sprintf("PID: %d - Proceso suspendido y guardado en swap", paquete.PID))
}

func ConcatenarDatosProceso(PID int) []byte {
	TablaPaginas, err := ObtenerTablaPaginas(PID)
	if err != nil {
		panic(err)
	}
	marcosAsignados := make([]int, 0)
	ObtenerMarcosAsignados(TablaPaginas, 1, &marcosAsignados)
	buffer := make([]byte, 0)
	for _, marco := range marcosAsignados {
		inicio := marco * ClientConfig.PAGE_SIZE
		fin := ClientConfig.PAGE_SIZE * (marco + 1)
		buffer = append(buffer, MemoriaDeUsuario[inicio:fin]...)
	}

	return buffer
}

func ObtenerTablaPaginas(PID int) (*NodoTablaPaginas, error) {

	for _, p := range ProcesosEnMemoria {
		if p.PID == PID {
			return p.TablaPaginas, nil
		}
	}

	return nil, fmt.Errorf("Proceso con PID %d no encontrado en memoria", PID)
}

func Crear_procesoPrueba(tamanio int, pid int) {
	TablaDePaginas := CrearTablaPaginas(1, ClientConfig.NUMBER_OF_LEVELS, ClientConfig.ENTRIES_PER_PAGE)

	// 2. Le asigno el espacio solicitado (si es posible)
	ReservarMemoria(tamanio, TablaDePaginas)

	// 3. Creo el proceso y lo guardo en la lista de procesos en memoria
	nuevoProceso := Proceso{
		PID:          pid,
		TablaPaginas: TablaDePaginas,
	}

	mutexProcesosEnMemoria.Lock()
	ProcesosEnMemoria = append(ProcesosEnMemoria, &nuevoProceso)
	mutexProcesosEnMemoria.Unlock()
}

func SuspenderProcesoPrueba(pid int) {
	buffer := new(bytes.Buffer) // Buffer de bytes
	encoder := gob.NewEncoder(buffer)

	datosProceso := ConcatenarDatosProceso(pid)
	procesoASuspeder := ProcesoSwap{
		PID:  pid,
		Data: datosProceso,
	}
	encoder.Encode(procesoASuspeder)
	data := buffer.Bytes()
	fmt.Println("Buffer bytes:", data)
	file, err := os.OpenFile(ClientConfig.SWAPFILE_PATH, os.O_APPEND, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	_, errWrite := file.Write(data) // data es []byte
	if errWrite != nil {
		panic(errWrite)
	}

	tablaDePaginas, err := ObtenerTablaPaginas(pid)
	if err != nil {
		slog.Error(fmt.Sprintf("Error al obtener la tabla de paginas del proceso %d: %v", pid, err))
	}
	DesasignarMarcos(tablaDePaginas, 1)

	slog.Info(fmt.Sprintf("PID: %d - Proceso suspendido y guardado en swap", pid))
}

/* eliminar entradas de swap

deserializar el archivo de swap

cuando encuentra el struct con el mismo PID entonces lo salyeo

guardo todo denuevo en el archivo de swap

*/

/*

type Persona struct {
    Edad   int32
    Altura int32
}

func main() {
    p := Persona{Edad: 30, Altura: 175}

    archivo, err := os.Create("persona.bin")
    if err != nil {
        log.Fatal(err)
    }
    defer archivo.Close()

    // Escribimos los campos uno por uno en binario
    err = binary.Write(archivo, binary.LittleEndian, p)
    if err != nil {
        log.Fatal("Error escribiendo:", err)
    }

    log.Println("Persona escrita en persona.bin")
}


archivo, err := os.Open("persona.bin")
// ...
var p Persona
err = binary.Read(archivo, binary.LittleEndian, &p)

*/
