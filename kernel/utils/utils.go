package utils

// Si el nombre de una funcion/variable empieza con una letra mayuscula, es porque es exportable
// Si empieza con una letra minuscula, es porque es privada al paquete

import (
	"encoding/json"
	"fmt"
	"globales"
	"globales/servidor"
	"log"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"
)

// --------- VARIABLES DEL KERNEL --------- //

var ClientConfig *Config

// ruta y tamaño del proceso inicial
var RutaInicial string
var TamanioInicial int

// Semaforos
var mutexColaNew sync.Mutex
var mutexColaReady sync.Mutex
var mutexColaRunning sync.Mutex
var mutexColaBlocked sync.Mutex
var mutexColaSuspendedBlocked sync.Mutex
var mutexColaSuspendedReady sync.Mutex
var mutexColaExit sync.Mutex
var mutexCrearPID sync.Mutex
var planificadorCortoPlazo sync.Mutex

// Conexiones CPU
var ConexionesCPU []globales.HandshakeCPU
var mutexConexionesCPU sync.Mutex

// Colas de los procesos
var ColaNew *[]*globales.PCB
var ColaReady *[]*globales.PCB
var ColaRunning *[]*globales.PCB
var ColaBlocked *[]*globales.PCB
var ColaSuspendedBlocked *[]*globales.PCB
var ColaSuspendedReady *[]*globales.PCB
var ColaExit *[]*globales.PCB

var PlanificadorActivo bool = false

var UltimoPID int = 0

var ProcesosBlocked []ProcesoSuspension

var algoritmoColaNew string
var algoritmoColaReady string
var alfa int
var estimadoInicial int

// lista de ios q se conectaron
var DispositivosIO []*DispositivoIO

// --------- ESTRUCTURAS DEL KERNEL --------- //
type Config struct {
	IP_MEMORY               string `json:"ip_memory"`
	PORT_MEMORY             int    `json:"port_memory"`
	IP_KERNEL               string `json:"ip_kernel"`
	PORT_KERNEL             int    `json:"port_kernel"`
	SCHEDULER_ALGORITHM     string `json:"scheduler_algorithm"`
	READY_INGRESS_ALGORITHM string `json:"ready_ingress_algorithm"`
	ALPHA                   int    `json:"alpha"`
	INITIAL_ESTIMATE        int    `json:"initial_estimate"`
	SUSPENSION_TIME         int    `json:"suspension_time"`
	LOG_LEVEL               string `json:"log_level"`
}

type Paquete struct {
	Valores string `json:"valores"`
}

type ProcesoSuspension struct {
	PID           int
	TiempoBloqueo time.Time
}

type PeticionMemoria struct {
	PID     int    `json:"pid"`
	Tamanio int    `json:"tamanio"`
	Ruta    string `json:"ruta"`
}

type RespuestaMemoria struct {
	Exito   bool   `json:"exito"`
	Mensaje string `json:"mensaje"`
}

type PeticionSwap struct {
	PID    int    `json:"pid"`
	Accion string `json:"accion"` // SWAP_OUT o SWAP_IN
}

type HandshakeIO struct {
	Nombre string `json:"nombre"`
	IP     string `json:"ip"`
	Puerto int    `json:"puerto"`
}

type DispositivoIO struct {
	Nombre         string
	IP             string
	Puerto         int
	Cola           []*ProcesoEsperandoIO // cola de peticiones de IO
	MutexCola      *sync.Mutex           // mutex para garantizar que entre un pcb a la vez a la cola
	EstaDisponible bool
	EstaConectado  bool
}

type RespuestaIO struct {
	PID                int    `json:"pid"`
	Nombre_Dispositivo string `json:"nombre_dispositivo"`
}

type PeticionIO struct {
	PID    int `json:"pid"`
	Tiempo int `json:"tiempo"`
}

type ProcesoEsperandoIO struct {
	PCB    *globales.PCB
	Tiempo int
}

// --------- FUNCIONES DEL KERNEL --------- //
func IniciarConfiguracion(filePath string) *Config {
	var config *Config
	configFile, err := os.Open(filePath)
	if err != nil {
		slog.Error(err.Error())
	}
	defer configFile.Close()

	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&config)

	algoritmoColaNew = config.READY_INGRESS_ALGORITHM
	algoritmoColaReady = config.SCHEDULER_ALGORITHM
	alfa = config.ALPHA
	estimadoInicial = config.INITIAL_ESTIMATE

	return config
}

func ValidarArgumentosKernel() (string, int) {
	if len(os.Args) < 2 {
		fmt.Println("Error: Falta el archivo de pseudocódigo")
		fmt.Println("Uso: ./kernel [archivo_pseudocodigo] [tamanio_proceso]")
		os.Exit(1)
	}

	if len(os.Args) < 3 {
		fmt.Println("Error: Falta el tamaño del proceso")
		fmt.Println("Uso: ./kernel [archivo_pseudocodigo] [tamanio_proceso]")
		os.Exit(1)
	}

	rutaInicial := os.Args[1]
	tamanio, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Println("Error: El tamaño del proceso debe ser un número entero")
		os.Exit(1)
	}
	return rutaInicial, tamanio
}

func InicializarColas() {
	ColaNew = &[]*globales.PCB{}
	ColaReady = &[]*globales.PCB{}
	ColaRunning = &[]*globales.PCB{}
	ColaBlocked = &[]*globales.PCB{}
	ColaSuspendedBlocked = &[]*globales.PCB{}
	ColaSuspendedReady = &[]*globales.PCB{}
	ColaExit = &[]*globales.PCB{}
}

func AgregarPCBaCola(pcb *globales.PCB, cola *[]*globales.PCB) {
	mutex, err := mutexCorrespondiente(cola)
	if err == nil {
		mutex.Lock()

		pcb.TiempoInicioEstado = time.Now()

		// Verificar si el PCB ya está en la cola
		for _, p := range *cola {
			if p.PID == pcb.PID {
				slog.Error("El PCB ya está en la cola")
				return
			}
		}

		// Verificar que el PCB no esté en estado EXIT
		if obtenerEstadoDeCola(cola) == "READY" && pcb.ME.EXIT > 0 {
			slog.Error(fmt.Sprintf("Intento de agregar PCB con PID %d a READY, pero ya está en EXIT", pcb.PID))
			return
		}

		*cola = append(*cola, pcb)

		mutex.Unlock()
		slog.Info(fmt.Sprintf("## (%d) agregado a la cola: %s", pcb.PID, obtenerEstadoDeCola(cola)))
		actualizarMetricasEstado(pcb, obtenerEstadoDeCola(cola))
	}

}

func mutexCorrespondiente(cola *[]*globales.PCB) (*sync.Mutex, error) {
	switch cola {
	case ColaNew:
		return &mutexColaNew, nil
	case ColaReady:
		return &mutexColaReady, nil
	case ColaRunning:
		return &mutexColaRunning, nil
	case ColaBlocked:
		return &mutexColaBlocked, nil
	case ColaSuspendedBlocked:
		return &mutexColaSuspendedBlocked, nil
	case ColaSuspendedReady:
		return &mutexColaSuspendedReady, nil
	case ColaExit:
		return &mutexColaExit, nil
	}
	return nil, fmt.Errorf("no existe mutex correspondiente")
}

func LeerPCBDesdeCola(cola *[]*globales.PCB) (*globales.PCB, error) {
	mutex, err := mutexCorrespondiente(cola)
	if err != nil {
		return &globales.PCB{}, fmt.Errorf("no existe mutex correspondiente")
	}

	if len(*cola) > 0 {
		mutex.Lock()
		pcb := (*cola)[0]
		*cola = (*cola)[1:]
		mutex.Unlock()

		tiempoTranscurrido := time.Since(pcb.TiempoInicioEstado).Milliseconds()
		actualizarMetricasTiempo(pcb, obtenerEstadoDeCola(cola), tiempoTranscurrido)

		slog.Debug(fmt.Sprintf("PCB leido desde la cola: %v", pcb))
		return pcb, nil
	} else {
		slog.Debug("No hay PCBs en la cola")
		return &globales.PCB{}, fmt.Errorf("no hay PCBs en la cola")
	}
}

func ReinsertarEnFrenteCola(cola *[]*globales.PCB, pcb *globales.PCB) {
	slicePCB := []*globales.PCB{pcb}
	mutex, err := mutexCorrespondiente(cola)
	mutex.Lock()
	if err != nil {
		return
	}
	*cola = append(slicePCB, *cola...)
	mutex.Unlock()
}

/*
func CambiarDeEstado(origen *[]*globales.PCB, destino *[]*globales.PCB) {
	pcb, err := LeerPCBDesdeCola(origen)
	if err == nil {
		// Actualizar el tiempo transcurrido en el estado anterior
		tiempoTranscurrido := time.Since(pcb.TiempoInicioEstado).Milliseconds()
		actualizarMetricasTiempo(pcb, obtenerEstadoDeCola(origen), tiempoTranscurrido)

		// Establecer el nuevo tiempo de inicio para el nuevo estado
		pcb.TiempoInicioEstado = time.Now()

		// Actualizar el contador de estado
		actualizarMetricasEstado(pcb, obtenerEstadoDeCola(destino))

		AgregarPCBaCola(pcb, destino)
		var nombreOrigen, nombreDestino = traducirNombresColas(origen, destino)
		slog.Info(fmt.Sprintf("## (%d) Pasa del estado %s al estado %s", pcb.PID, nombreOrigen, nombreDestino)) // log obligatorio
	} else {
		slog.Info(fmt.Sprintf("No hay PCBs en la cola %v", origen))
	}
}
*/

/*
func traducirNombresColas(origen *[]*globales.PCB, destino *[]*globales.PCB) (string, string) {
	var nombreOrigen string = ""
	var nombreDestino string = ""
	nombreOrigen = obtenerEstadoDeCola(origen)
	nombreDestino = obtenerEstadoDeCola(destino)
	return nombreOrigen, nombreDestino
}
*/

func obtenerEstadoDeCola(cola *[]*globales.PCB) string {
	switch cola {
	case ColaNew:
		return "NEW"
	case ColaReady:
		return "READY"
	case ColaRunning:
		return "RUNNING"
	case ColaBlocked:
		return "BLOCKED"
	case ColaSuspendedBlocked:
		return "SUSPENDED_BLOCKED"
	case ColaSuspendedReady:
		return "SUSPENDED_READY"
	case ColaExit:
		return "EXIT"
	}
	return ""
}

func actualizarMetricasTiempo(pcb *globales.PCB, estado string, tiempoMS int64) {
	slog.Info(fmt.Sprintf("Actualizando métricas de tiempo para el PCB %d en estado %s con tiempo %d ms", pcb.PID, estado, tiempoMS))
	switch estado {
	case "NEW":
		pcb.MT.NEW += int(tiempoMS)
	case "READY":
		pcb.MT.READY += int(tiempoMS)
	case "RUNNING":
		pcb.MT.RUNNING += int(tiempoMS)
	case "BLOCKED":
		pcb.MT.BLOCKED += int(tiempoMS)
	case "SUSPENDED_BLOCKED":
		pcb.MT.SUSPENDED_BLOCKED += int(tiempoMS)
	case "SUSPENDED_READY":
		pcb.MT.SUSPENDED_READY += int(tiempoMS)
	case "EXIT":
		pcb.MT.EXIT += int(tiempoMS)
	}
}

func actualizarMetricasEstado(pcb *globales.PCB, estado string) {
	switch estado {
	case "NEW":
		pcb.ME.NEW++
	case "READY":
		pcb.ME.READY++
	case "RUNNING":
		pcb.ME.RUNNING++
	case "BLOCKED":
		pcb.ME.BLOCKED++
	case "SUSPENDED_BLOCKED":
		pcb.ME.SUSPENDED_BLOCKED++
	case "SUSPENDED_READY":
		pcb.ME.SUSPENDED_READY++
	case "EXIT":
		pcb.ME.EXIT++
	}
}

func buscarPCBYSacarDeCola(pid int, cola *[]*globales.PCB) (*globales.PCB, error) {
	//TODO: actualizar metricas de tiempo
	mutex, err := mutexCorrespondiente(cola)
	if err != nil {
		slog.Info("No existe la cola solicitada")
		return &globales.PCB{}, fmt.Errorf("no se ha encontrado el PCB")
	}
	mutex.Lock()
	for i, p := range *cola {
		if p.PID == pid {
			pcb := p

			// Actualizar el tiempo transcurrido en el estado anterior
			tiempoTranscurrido := time.Since(pcb.TiempoInicioEstado).Milliseconds()
			actualizarMetricasTiempo(pcb, obtenerEstadoDeCola(cola), tiempoTranscurrido)

			// lo saco de bloqueados
			*cola = append((*cola)[:i], (*cola)[i+1:]...)
			mutex.Unlock()

			// Establecer el nuevo tiempo de inicio para el nuevo estado
			// pcb.TiempoInicioEstado = time.Now()

			return pcb, nil
		}
	}
	mutex.Unlock()

	slog.Info(fmt.Sprintf("No se encontró el PCB del PID %d en la cola %s", pid, obtenerEstadoDeCola(cola)))
	return &globales.PCB{}, fmt.Errorf("no se ha encontrado el PCB")
}

func AtenderCPU(w http.ResponseWriter, r *http.Request) {
	var paquete servidor.PCB = servidor.RecibirPaquetesCpu(w, r)
	slog.Info("Recibido paquete CPU")
	log.Printf("%+v\n", paquete)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func RecibirProcesoInterrumpido(w http.ResponseWriter, r *http.Request) {
	paquete := globales.Interrupcion{}
	paquete = servidor.DecodificarPaquete(w, r, &paquete)

	pcb, err := buscarPCBYSacarDeCola(paquete.PID, ColaRunning) // Saco el proceso de la cola de Running
	if err != nil {
		slog.Error(fmt.Sprintf("No se encontró el PCB del PID %d en la cola", paquete.PID))
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Pcb no encontrado"))
	}

	pcb.PC = paquete.PC
	AgregarPCBaCola(pcb, ColaReady)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func AtenderHandshakeCPU(w http.ResponseWriter, r *http.Request) {
	var paquete globales.HandshakeCPU = servidor.DecodificarPaquete(w, r, &globales.HandshakeCPU{})
	slog.Info("Recibido handshake CPU.")

	mutexConexionesCPU.Lock() // bloquea
	ConexionesCPU = append(ConexionesCPU, paquete)
	mutexConexionesCPU.Unlock() // desbloquea

	log.Printf("%+v\n", paquete)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func IniciarProceso(w http.ResponseWriter, r *http.Request) {
	paquete := globales.SolicitudProceso{}
	paquete = servidor.DecodificarPaquete(w, r, &paquete)

	CrearProceso(paquete.ARCHIVO_PSEUDOCODIGO, paquete.TAMAÑO_PROCESO)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func TerminarProceso(w http.ResponseWriter, r *http.Request) {
	pid := globales.PID{}
	pid = servidor.DecodificarPaquete(w, r, &pid)
	slog.Debug(fmt.Sprintf("Finalizando proceso (terminar proceso) con PID: %d", pid))
	//planificadorCortoPlazo.Lock()
	FinalizarProceso(pid.NUMERO_PID, ColaRunning)
	//planificadorCortoPlazo.Unlock()
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))

}

func FinalizarProceso(pid int, cola *[]*globales.PCB) {
	slog.Debug(fmt.Sprintf("Cola READY (finalizar proceso): %v \n", &ColaReady))
	slog.Debug(fmt.Sprintf("Cola RUNNING (finalizar proceso): %v \n", &ColaRunning))
	slog.Debug(fmt.Sprintf("Cola EXIT (finalizar proceso): %v \n", &ColaExit))

	slog.Debug(fmt.Sprintf("Finalizando proceso (finalizar proceso) con PID: %d", pid))
	pcb, err := buscarPCBYSacarDeCola(pid, cola)
	if err != nil {
		slog.Error(fmt.Sprintf("No se encontró el PCB del PID %d en la cola", pid))
	} else {
		// Calcular el tiempo transcurrido en el estado actual
		//tiempoTranscurrido := time.Since(pcb.TiempoInicioEstado).Milliseconds()
		//actualizarMetricasTiempo(pcb, obtenerEstadoDeCola(cola), tiempoTranscurrido)

		// conexion con memoria para liberar espacio del PCB
		pid_a_eliminar := globales.PIDAEliminar{
			NUMERO_PID: pid,
			TAMANIO:    pcb.Tamanio,
		}
		// peticion a memoria para liberar el espacio
		globales.GenerarYEnviarPaquete(&pid_a_eliminar, ClientConfig.IP_MEMORY, ClientConfig.PORT_MEMORY, "/kernel/liberar_memoria")

		//pcb.TiempoInicioEstado = time.Now()
		AgregarPCBaCola(pcb, ColaExit)

		// cambio de estado a Exit del PCB
		slog.Info(fmt.Sprintf("## (%d) - Finaliza el proceso \n", pid)) // log obligatorio de Fin proceso

		ImprimirMetricasProceso(*pcb)
	}
}

func DumpearMemoria(w http.ResponseWriter, r *http.Request) {
	paquete := globales.SolicitudDump{}
	paquete = servidor.DecodificarPaquete(w, r, &paquete)

	pidABloquear := paquete.PID
	pc := paquete.PC

	pcbABloquear, err := buscarPCBYSacarDeCola(pidABloquear, ColaRunning)
	if err != nil {
		slog.Error(fmt.Sprintf("No se encontró el PCB del PID %d en la cola", pidABloquear))
	}

	pcbABloquear.PC = pc

	AgregarPCBaCola(pcbABloquear, ColaBlocked)
	slog.Info(fmt.Sprintf("## (%d) Pasa del estado RUNNING al estado BLOCKED", pidABloquear))

	var peticionEnviada, _ = globales.GenerarYEnviarPaquete(&paquete, ClientConfig.IP_MEMORY, ClientConfig.PORT_MEMORY, "/kernel/dump_de_proceso")

	if peticionEnviada.StatusCode == 200 { // me llega fin de operacion de memoria
		// desbloqueo el proceso y lo envio a ready
		pcbADesbloquear, err := buscarPCBYSacarDeCola(pidABloquear, ColaBlocked)
		if err != nil {
			pcbADesbloquear, err = buscarPCBYSacarDeCola(pidABloquear, ColaSuspendedBlocked)
			if err == nil {
				AgregarPCBaCola(pcbADesbloquear, ColaSuspendedReady)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ok"))
			}
		} else {
			AgregarPCBaCola(pcbADesbloquear, ColaReady)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		}
	} else {
		FinalizarProceso(pidABloquear, ColaBlocked) // en caso de error --> exit
	}

}

func CrearProceso(rutaPseudocodigo string, tamanio int) {
	mutexCrearPID.Lock()
	pid := UltimoPID
	UltimoPID++
	mutexCrearPID.Unlock()
/*
	archivoProceso := globales.MEMORIA_CREACION_PROCESO{ // Ida y vuelta con memoria
		PID:                     pid,
		RutaArchivoPseudocodigo: rutaPseudocodigo,
		Tamanio:                 tamanio,
	}
	globales.GenerarYEnviarPaquete(&archivoProceso, ClientConfig.IP_MEMORY, ClientConfig.PORT_MEMORY, "/kernel/archivoProceso")
*/
	// espera 1 segundo para simular la creación del proceso
	time.Sleep(1 * time.Second)

	slog.Info(fmt.Sprintf("## (%d) Se crea el proceso - Estado: NEW", pid))

	pcb := globales.PCB{
		PID:                pid,
		PC:                 0,
		RutaPseudocodigo:   rutaPseudocodigo,
		Tamanio:            tamanio,
		TiempoInicioEstado: time.Now(),
		ME:                 globales.METRICAS_KERNEL{}, // por defecto go inicializa todo en 0
		MT:                 globales.METRICAS_KERNEL{},
	}

	AgregarPCBaCola(&pcb, ColaNew)
	ordenarColaNew()
}

func ordenarColaNew() {
	mutexColaNew.Lock()

	if algoritmoColaNew == "FIFO" {
		slog.Debug("Ordenando cola NEW por FIFO")
	} else if algoritmoColaNew == "PMCP" {
		sort.Slice(*ColaNew, func(i, j int) bool {
			return (*ColaNew)[i].Tamanio < (*ColaNew)[j].Tamanio
		})
	}
/*
	for _, p := range *ColaNew {
		slog.Debug(fmt.Sprintf("Pid: (%d) , %d \n", p.PID, p.Tamanio))
	}
*/
	mutexColaNew.Unlock()

}

func ordenarColaSuspendedReady() {
	mutexColaSuspendedReady.Lock()

	if algoritmoColaNew == "FIFO" {
		slog.Debug("Ordenando cola NEW por FIFO")
	} else if algoritmoColaNew == "PMCP" {
		sort.Slice(*ColaSuspendedReady, func(i, j int) bool {
			return (*ColaSuspendedReady)[i].Tamanio < (*ColaSuspendedReady)[j].Tamanio
		})
	}
/*
	for _, p := range *ColaSuspendedReady {
		slog.Debug(fmt.Sprintf("Pid: (%d) , %d \n", p.PID, p.Tamanio))
	}
*/
	mutexColaSuspendedReady.Unlock()

}

func ordenarColaReady() {
	mutexColaReady.Lock()

	mutexColaReady.Unlock()
}

// envia peticion a memoria para q mueva un proceso a swap
func EnviarProcesoASwap(pcb globales.PCB) bool {
	//peticion := PeticionSwap{
	//	PID: pcb.PID,
	//	Accion: "SWAP_OUT",
	//}

	//ip := ClientConfig.IP_MEMORY
	//puerto := ClientConfig.PORT_MEMORY

	// todavia no existe pero deberia ser algo parecido
	//globales.GenerarYEnviarPaquete(&peticion, ip, puerto, "/memoria/swap")

	return true
}

// pide a memoria inicializar un proceso
func InicializarProcesoEnMemoria(pcb globales.PCB) bool {
	//peticion := PeticionMemoria{
	//	PID:     pcb.PID,
	//	Tamanio: pcb.tamanio,
	//	Ruta:    pcb.rutaPseudocodigo,
	//}

	//ip := ClientConfig.IP_MEMORY
	//puerto := ClientConfig.PORT_MEMORY

	// por ahora lo simulo con un return true
	return true
}

// inicia todos los planificadores
func IniciarPlanificadores() {
	PlanificadorActivo = true

	go PlanificadorLargoPlazo()
	go PlanificadorCortoPlazo()
	go PlanificadorMedianoPlazo()

	slog.Info("Planificadores iniciados: largo, corto y mediano plazo")
}

// atiende la cola de new y pasa a ready si hay espacio
func PlanificadorLargoPlazo() {
	for PlanificadorActivo {
		// primero miro suspendidos ready, que tiene prioridad por sobre ready
		atenderColaSuspendidosReady()

		// si no hay procesos en new, sigo
		if len(*ColaNew) == 0 {
			continue
		}

		pcb, err := LeerPCBDesdeCola(ColaNew)
		if err != nil {
			continue
		}
		//tiempoTranscurrido := time.Since(pcb.TiempoInicioEstado).Milliseconds()
		////actualizarMetricasTiempo(pcb, "NEW", tiempoTranscurrido) // actualizo el tiempo en new

		// siempre retorna true por ahora
		inicializado := InicializarProcesoEnMemoria(*pcb) // propagar la ruta y el tamaño con el que se ejecuta cuando no tengamos que mockear la resp de memoria

		if inicializado {
			// actualizo metricas
			//pcb.ME.NEW--
			//pcb.ME.READY++

			// lo paso a ready
			AgregarPCBaCola(pcb, ColaReady)
			slog.Info(fmt.Sprintf("## (%d) Pasa del estado NEW al estado READY", pcb.PID))
		} else {
			// no pudo inicializarse, vuelve a new
			AgregarPCBaCola(pcb, ColaNew)
		}
	}
}

// si hay procesos suspendidos ready intenta pasarlos a ready
func atenderColaSuspendidosReady() {
	if len(*ColaSuspendedReady) == 0 {
		return
	}

	pcb, err := LeerPCBDesdeCola(ColaSuspendedReady)
	if err != nil {
		return
	}

	// siempre true por ahora
	inicializado := true

	if inicializado {
		AgregarPCBaCola(pcb, ColaReady) // lo paso a ready
		slog.Info(fmt.Sprintf("## (%d) Pasa del estado SUSPENDED_READY al estado READY", pcb.PID))
	} else {
		// no se pudo, vuelve a cola
		AgregarPCBaCola(pcb, ColaSuspendedReady)
	}
}

// recibe proceso que retorna de cpu
func AtenderRetornoCPU(w http.ResponseWriter, r *http.Request) {
	paquete := globales.PeticionCPU{}
	paquete = servidor.DecodificarPaquete(w, r, &paquete)

	// busco el pcb en running por su pid
	var pcbEncontrado bool = false
	var pcb globales.PCB

	for i, p := range *ColaRunning {
		if p.PID == paquete.PID {
			pcb = *p
			// actualizo el pc con valor retornado
			pcb.PC = paquete.PC
			// lo saco de running
			*ColaRunning = append((*ColaRunning)[:i], (*ColaRunning)[i+1:]...)
			pcbEncontrado = true
			break
		}
	}

	if !pcbEncontrado {
		slog.Error(fmt.Sprintf("No encuentro el PCB %d en cola running", paquete.PID))
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("pcb no encontrado"))
		return
	}

	// Actualizar tiempo en el estado RUNNING
	tiempoTranscurrido := time.Since(pcb.TiempoInicioEstado).Milliseconds()
	actualizarMetricasTiempo(&pcb, "RUNNING", tiempoTranscurrido)

	// Actualizar estado a EXIT
	//pcb.TiempoInicioEstado = time.Now()
	//actualizarMetricasEstado(&pcb, "EXIT")

	AgregarPCBaCola(&pcb, ColaExit)
	slog.Info(fmt.Sprintf("## (%d) Pasa del estado RUNNING al estado EXIT", pcb.PID))

	// Imprimir las métricas del proceso finalizado
	slog.Info(fmt.Sprintf("## (%d) - Finaliza el proceso", pcb.PID))
	ImprimirMetricasProceso(pcb)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// planificador de corto plazo fifo
func PlanificadorCortoPlazo() {
	slog.Info("antes del for corto plazo")
	if PlanificadorActivo {
		for {
			slog.Debug("Arranca el for") // Borrar

			slog.Debug(fmt.Sprint("Cola de ready: ", ColaReady)) // Borrar

			mutexConexionesCPU.Lock()
			slog.Debug("BLOQUEA LAS CONEXIONES") // Borrar
			hayCPUsDisponibles := len(ConexionesCPU) > 0
			mutexConexionesCPU.Unlock()
			slog.Debug("LIBERA LAS CONEXIONES") // Borrar
			// si no hay cpus disponibles, sigo
			if !hayCPUsDisponibles {
				slog.Debug("No hay CPUs disponibles")
				continue
			}

			// si no hay procesos ready, sigo
			if len(*ColaReady) == 0 {
				slog.Debug("La cola de READY esta vacia")
				continue
			}

			slog.Info("Antes de iniciar planificador corto plazo")
			time.Sleep(10000)
			PlanificarSiguienteProceso()
		}
	}
}

func BuscarCPULibre() (globales.HandshakeCPU, error) {
	mutexConexionesCPU.Lock()

	if len(ConexionesCPU) == 0 {
		mutexConexionesCPU.Unlock()
		return globales.HandshakeCPU{}, fmt.Errorf("no hay CPUs disponibles")
	}

	// selecciona la primera cpu disponible
	cpu := ConexionesCPU[0]
	ConexionesCPU = ConexionesCPU[1:]

	mutexConexionesCPU.Unlock()

	return cpu, nil
}

func EnviarProcesoACPU(pcb *globales.PCB, cpu globales.HandshakeCPU) {
	peticionCPU := globales.PeticionCPU{
		PID: pcb.PID,
		PC:  pcb.PC,
	}

	ip := cpu.IP_CPU
	puerto := cpu.PORT_CPU
	url := fmt.Sprintf("/cpu/%s/ejecutarProceso", cpu.ID_CPU)

	// Actualizar el tiempo en el estado READY
	//tiempoTranscurrido := time.Since(pcb.TiempoInicioEstado).Milliseconds()
	//actualizarMetricasTiempo(&pcb, "READY", tiempoTranscurrido)

	slog.Debug("Intentando enviar pcb a cpu ...")

	slog.Info(fmt.Sprintf("## (%d) Pasa del estado READY al estado RUNNING", pcb.PID))
	AgregarPCBaCola(pcb, ColaRunning)
	// no mover esto a abajo del generar y enviar paquete, porque se rompe

	resp, _ := globales.GenerarYEnviarPaquete(&peticionCPU, ip, puerto, url)
	if resp.StatusCode != 200 {
		slog.Error(fmt.Sprintf("Error al enviar el proceso a la CPU: %s", resp.Status))
		ReinsertarEnFrenteCola(ColaReady, pcb)
		return
	}

	//slog.Info("ANTES DE ACTUALIZAR TIEMPO EN READY - ENVIAR PROCESO A CPU")

	//slog.Info(fmt.Sprintf("Cola de READY: %v", &ColaReady))

	// slog.Info(fmt.Sprintf("Cola de RUNNING: %v", &ColaRunning))
}

func PlanificarSiguienteProceso() { // planifica el siguiente proceso
	switch algoritmoColaReady {
	case "FIFO":
		slog.Debug("Antes de FIFO")
		planificarSinDesalojo()
	case "SJF":
		planificarSinDesalojo() // sin desalojo
	case "SRT":
		//planificarConDesalojo() // con desalojo
	default:
		planificarSinDesalojo()
	}
}

func planificarSinDesalojo() {

	for {

		planificadorCortoPlazo.Lock()
		pcb, err := LeerPCBDesdeCola(ColaReady)
		if err != nil {
			//ReinsertarEnFrenteCola(&ColaReady, pcb)

			planificadorCortoPlazo.Unlock()
			slog.Debug("No hay procesos listos") // borrar
			continue
		}

		slog.Debug(fmt.Sprintf("Conexiones CPU del sistema: %v", ConexionesCPU))

		cpuLibre, err := BuscarCPULibre() // busco una cpu disponible
		if err != nil {
			ReinsertarEnFrenteCola(ColaReady, pcb)
			planificadorCortoPlazo.Unlock()
			slog.Debug("No hay CPUs disponibles")
			continue
		}
		// EnviarProcesoACPU(pcb, cpuLibre)
		planificadorCortoPlazo.Unlock()

		EnviarProcesoACPU(pcb, cpuLibre)
	}
}
/*
func planificarConDesalojo() {
	for {
		planificadorCortoPlazo.Lock()

		// Buscar todas las CPUs libres
		mutexConexionesCPU.Lock()
		cpusLibres := make([]globales.HandshakeCPU, len(ConexionesCPU))
		copy(cpusLibres, ConexionesCPU)
		mutexConexionesCPU.Unlock()

		// Mientras haya CPUs libres y procesos en READY, planificar
		for len(cpusLibres) > 0 && len(*ColaReady) > 0 {
			pcbReady := obtenerMenorEstimadoDeReady()
			if pcbReady == nil {
				break
			}
			pcbReady, err := buscarPCBYSacarDeCola(pcbReady.PID, ColaReady)
			if err != nil {
				break
			}
			// Tomar una CPU libre
			cpuLibre := cpusLibres[0]
			cpusLibres = cpusLibres[1:]

			EnviarProcesoACPU(pcbReady, cpuLibre)
			AgregarPCBaCola(pcbReady, ColaRunning)
		}

		// Si no hay CPUs libres, evaluar desalojo
		if len(*ColaReady) > 0 {
			pcbReady := obtenerMenorEstimadoDeReady()
			pcbMasLento := obtenerMayorEstimadoDeRunning()
			if pcbReady != nil && pcbMasLento != nil && pcbReady.EstimadoActual < pcbMasLento.EstimadoActual {
				cpuEjecutando := CPUQueEjecuta(pcbMasLento.PID)
				if cpuEjecutando != nil {
					slog.Info(fmt.Sprintf("Desalojando PID %d para ejecutar PID %d", pcbMasLento.PID, pcbReady.PID))
					InterrumpirProceso(pcbMasLento, *cpuEjecutando)
				}
			}
		}

		planificadorCortoPlazo.Unlock()
		time.Sleep(100 * time.Millisecond)
	}
}

func InterrumpirProceso(pcb *globales.PCB, cpu globales.HandshakeCPU) {
	slog.Info(fmt.Sprintf("Enviando interrupción a CPU %s para desalojar PID %d", cpu.ID_CPU, pcb.PID))

	interrupcion := globales.Interrupcion{
		PID:    pcb.PID,
		PC:     pcb.PC, // Podés enviar el PC actual si lo necesitás
		MOTIVO: "Desalojo por SRT",
	}

	resp, err := globales.GenerarYEnviarPaquete(&interrupcion, cpu.IP_CPU, cpu.PORT_CPU, "/cpu/desalojarProceso")
	if err != nil || resp.StatusCode != 200 {
		slog.Error(fmt.Sprintf("Error al interrumpir proceso %d en CPU %s: %v", pcb.PID, cpu.ID_CPU, err))
		return
	}

	slog.Info(fmt.Sprintf("Interrupción enviada correctamente a CPU %s para PID %d", cpu.ID_CPU, pcb.PID))
}

/*
func CPUQueEjecuta(pid int) *globales.HandshakeCPU {
	mutexConexionesCPU.Lock()
	defer mutexConexionesCPU.Unlock()
	for i := range ConexionesCPU {
		if ConexionesCPU[i].PID_EJECUTANDO == pid {
			return &ConexionesCPU[i]
		}
	}
	return nil
}

// Devuelve el PCB con menor estimado de la cola READY
func obtenerMenorEstimadoDeReady() *globales.PCB {
	mutexColaReady.Lock()
	defer mutexColaReady.Unlock()
	if len(*ColaReady) == 0 {
		return nil
	}
	ordenarColaReady()
	return (*ColaReady)[0]
}

// Devuelve el PCB con mayor estimado de la cola RUNNING
func obtenerMayorEstimadoDeRunning() *globales.PCB {
	mutexColaRunning.Lock()
	defer mutexColaRunning.Unlock()
	if len(*ColaRunning) == 0 {
		return nil
	}
	max := (*ColaRunning)[0]
	for _, p := range *ColaRunning {
		if p.EstimadoActual > max.EstimadoActual {
			max = p
		}
	}
	return max
}
*/
// planificador de mediano plazo
func PlanificadorMedianoPlazo() {
	for PlanificadorActivo {
		// reviso cada proceso bloqueado
		for i := 0; i < len(ProcesosBlocked); i++ {
			// calculo tiempo bloqueado
			tiempoBloqueo := time.Since(ProcesosBlocked[i].TiempoBloqueo)

			// si excede el tiempo de suspension lo suspendo
			if tiempoBloqueo.Milliseconds() > int64(ClientConfig.SUSPENSION_TIME) {
				pid := ProcesosBlocked[i].PID

				/*
					for j, p := range *ColaBlocked {
						if p.PID == pid {
							pcb = *p
							// lo saco d bloqueados
							(*ColaBlocked) = append((*ColaBlocked)[:j], (*ColaBlocked)[j+1:]...)
							pcbEncontrado = true
							break
						}
					} */

				pcb, err := buscarPCBYSacarDeCola(pid, ColaBlocked)
				if err != nil {
					ReinsertarEnFrenteCola(ColaBlocked, pcb)
					slog.Error(fmt.Sprintf("No se encontró el PCB del PID %d en la cola", pid))
				}

				if err == nil {
					// actualizo metricas
					//pcb.ME.BLOCKED--
					//pcb.ME.SUSPENDED_BLOCKED++

					// informo a memoria q lo mueva a swap
					swapExitoso := EnviarProcesoASwap(*pcb)

					if swapExitoso {
						// lo paso a susp_blocked
						AgregarPCBaCola(pcb, ColaSuspendedBlocked)
						slog.Info(fmt.Sprintf("## (%d) Pasa del estado BLOCKED al estado SUSPENDED_BLOCKED", pcb.PID))

						// elimino el registro de tiempo de bloqueo
						ProcesosBlocked = append(ProcesosBlocked[:i], ProcesosBlocked[i+1:]...)
						i-- // ajusto i porque eliminé un elemento del slice
					} else {
						// si falla lo vuelvo a poner en bloqueados
						AgregarPCBaCola(pcb, ColaBlocked)
					}
				}
			}
		}
	}
}

func ImprimirMetricasProceso(pcb globales.PCB) {
	slog.Info(fmt.Sprintf("## (%d) - Métricas de estado: NEW (%d) (%d), READY (%d) (%d), RUNNING (%d) (%d), BLOCKED (%d) (%d), SUSPENDED_BLOCKED (%d) (%d), SUSPENDED_READY (%d) (%d), EXIT (%d) (%d)",
		pcb.PID,
		pcb.ME.NEW, pcb.MT.NEW,
		pcb.ME.READY, pcb.MT.READY,
		pcb.ME.RUNNING, pcb.MT.RUNNING,
		pcb.ME.BLOCKED, pcb.MT.BLOCKED,
		pcb.ME.SUSPENDED_BLOCKED, pcb.MT.SUSPENDED_BLOCKED,
		pcb.ME.SUSPENDED_READY, pcb.MT.SUSPENDED_READY,
		pcb.ME.EXIT, pcb.MT.EXIT))
}

//TODO separar en archivos

// hacer una funcion que le pases por parametro un DispositivoIO

func mandarProcesoAIO(dispositivoIO *DispositivoIO) {
	slog.Info(fmt.Sprintf("## Dispositivo IO %s inicio goroutine", dispositivoIO.Nombre))
	cola := &dispositivoIO.Cola
	ipIO := dispositivoIO.IP
	puertoIO := dispositivoIO.Puerto

	for dispositivoIO.EstaConectado {
		if dispositivoIO.EstaDisponible {
			time.Sleep(1 * time.Second) // espera 1 segundo antes de verificar la cola nuevamente
			slog.Debug(fmt.Sprintf("## Dispositivo IO %s revisando cola %v", dispositivoIO.Nombre, (*cola)))

			if len(*cola) > 0 {

				dispositivoIO.MutexCola.Lock()
				proceso := (*cola)[0]
				(*cola) = (*cola)[1:]
				dispositivoIO.MutexCola.Unlock()
				dispositivoIO.EstaDisponible = false
				peticionEnviada := EnviarPeticionIO(proceso.PCB, ipIO, puertoIO, proceso.Tiempo)

				slog.Debug(fmt.Sprintf("## valor de peticion enviada: %t", peticionEnviada))

				if !peticionEnviada {
					slog.Error(fmt.Sprintf("## Error al enviar la peticion de IO al dispositivo %s", dispositivoIO.Nombre))
					FinalizarProceso(proceso.PCB.PID, ColaBlocked)
					dispositivoIO.EstaDisponible = true
				}
			}
		}
	}
}

func SolicitarIO(w http.ResponseWriter, r *http.Request) {
	var dispositivoEncontrado bool = false
	var ioDevice *DispositivoIO

	paquete := globales.SolicitudIO{}
	paquete = servidor.DecodificarPaquete(w, r, &paquete)

	slog.Info(fmt.Sprintf("Recibido solicitud de syscall IO: %s", paquete.NOMBRE))
	log.Printf("%+v\n", paquete)

	nombreIO := paquete.NOMBRE
	//tiempoBloqueo := paquete.TIEMPO
	pidABloquear := paquete.PID

	//Buscar el dispositivo por nombre
	for _, dispositivo := range DispositivosIO {
		if dispositivo.Nombre == nombreIO {
			ioDevice = dispositivo
			dispositivoEncontrado = true
			break
		}
	}

	// Verficar si el DispositivoIO existe
	if !dispositivoEncontrado {
		slog.Error(fmt.Sprintf("No encuentro el dispositivo IO %s", nombreIO))
		FinalizarProceso(pidABloquear, ColaRunning)
		return
	}

	pcbABloquear, err := buscarPCBYSacarDeCola(pidABloquear, ColaRunning)
	if err != nil {
		slog.Error(fmt.Sprintf("No se encontró el PCB del PID %d a bloquear en la cola", pidABloquear))
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Error"))
		return
	}

	AgregarPCBaCola(pcbABloquear, ColaBlocked)

	slog.Info(fmt.Sprintf("## (%d) - Bloqueado por IO: %s", pcbABloquear.PID, nombreIO)) // log obligatorio
	slog.Info(fmt.Sprintf("## (%d) Pasa del estado RUNNING al estado BLOCKED", pcbABloquear.PID))

	(*pcbABloquear).PC = paquete.PC

	procesoEsperandoIO := ProcesoEsperandoIO{
		PCB:    pcbABloquear,
		Tiempo: paquete.TIEMPO,
	}

	ioDevice.MutexCola.Lock()
	ioDevice.Cola = append(ioDevice.Cola, &procesoEsperandoIO) // Agregar el proceso a la cola del dispositivo IO
	slog.Debug(fmt.Sprintf("## (%d) - Agregado a la cola del dispositivo IO %s", pcbABloquear.PID, nombreIO))
	slog.Debug(fmt.Sprintf("## Cola del dispositivo IO %s: %+v", nombreIO, ioDevice.Cola))
	ioDevice.MutexCola.Unlock()

	// peticionEnviada := EnviarPeticionIO(pidABloquear, nombreIO, tiempoBloqueo) // se encarga tambien de eliminar de running y agregar a blocked
	// peticionEnviada := EnviarPeticionIO(pcbABloquear, nombreIO, tiempoBloqueo) // se encarga tambien de eliminar de running y agregar a blocked

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// guarda los IO q se conectan
func AtenderHandshakeIO(w http.ResponseWriter, r *http.Request) {
	paquete := HandshakeIO{}
	paquete = servidor.DecodificarPaquete(w, r, &paquete)

	slog.Info(fmt.Sprintf("Recibido handshake del dispositivo IO: %s", paquete.Nombre))

	dispositivoIO := DispositivoIO{
		Nombre:         paquete.Nombre,
		IP:             paquete.IP,
		Puerto:         paquete.Puerto,
		Cola:           make([]*ProcesoEsperandoIO, 0),
		MutexCola:      new(sync.Mutex),
		EstaDisponible: true,
		EstaConectado:  true,
	}

	DispositivosIO = append(DispositivosIO, &dispositivoIO)
	log.Printf("Dispositivo IO registrado: %+v\n", paquete)

	go mandarProcesoAIO(&dispositivoIO)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// envia peticion al dispositivo io disponible
func EnviarPeticionIO(pcbABloquear *globales.PCB, ipIO string, puertoIO int, tiempoIO int) bool {

	peticion := PeticionIO{ // armo el paquete con pid y tiempo
		PID:    pcbABloquear.PID,
		Tiempo: tiempoIO,
	}

	// guardo el tiempo en q se bloqueo para q el plani de mediano plazo lo suspenda
	registroSuspension := ProcesoSuspension{
		PID:           peticion.PID,
		TiempoBloqueo: time.Now(),
	}
	ProcesosBlocked = append(ProcesosBlocked, registroSuspension)

	//globales.GenerarYEnviarPaquete(&peticion, ipIO, puertoIO, "/io/peticion")           // mando la peticion al io
	resp, _ := globales.GenerarYEnviarPaquete(&peticion, ipIO, puertoIO, "/io/peticion") // mando la peticion al io

	if resp.StatusCode != 200 {
		return false
	} else {
		return true
	}
}

func AtenderFinIOPeticion(w http.ResponseWriter, r *http.Request) {
	paquete := RespuestaIO{}
	paquete = servidor.DecodificarPaquete(w, r, &paquete)

	slog.Info(fmt.Sprintf("## (%d) finalizó IO y pasa a READY", paquete.PID))

	// busco el pcb en bloqueados

	pidFinIO := paquete.PID
	nombreIO := paquete.Nombre_Dispositivo

	for _, dispositivo := range DispositivosIO {
		if (dispositivo).Nombre == nombreIO {
			dispositivo.EstaDisponible = true
			break
		}
	}

	pcb, err := buscarPCBYSacarDeCola(pidFinIO, ColaSuspendedBlocked)

	//TODO: revisar si evaluar primero cola blocked o suspended blocked

	if err == nil {
		AgregarPCBaCola(pcb, ColaSuspendedReady)

		slog.Info(fmt.Sprintf("## (%d) Pasa del estado SUSPENDED_BLOCKED al estado SUSPENDED_READY", pcb.PID))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))

		/*
			// busco y elimino el registro de tiempo de bloqueo
			for i, p := range ProcesosBlocked {
				if p.PID == paquete.PID {
					//TODO: agregar mutex procesos blocked
					ProcesosBlocked = append(ProcesosBlocked[:i], ProcesosBlocked[i+1:]...)
					break
				}
			}
		*/

		return
	} else {
		pcb, err := buscarPCBYSacarDeCola(pidFinIO, ColaBlocked)
		if err == nil {
			AgregarPCBaCola(pcb, ColaReady)
			slog.Info(fmt.Sprintf("## (%d) Pasa del estado BLOCKED al estado READY", pcb.PID))
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))

			/*
				// busco y elimino el registro de tiempo de bloqueo
				for i, p := range ProcesosBlocked {
					if p.PID == paquete.PID {
						//TODO: agregar mutex procesos blocked
						ProcesosBlocked = append(ProcesosBlocked[:i], ProcesosBlocked[i+1:]...)
						break
					}
				}
			*/
			return
		} else {
			slog.Error(fmt.Sprintf("No se encontró el PCB del PID %d en las colas blocked/suspended_Blocked", pidFinIO))
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("pcb no encontrado"))
			return
		}
	}

}
