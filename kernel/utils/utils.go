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

// --------- ESTRUCTURAS DEL KERNEL --------- //

type PCB struct {
	PID                                int             `json:"pid"`
	PC                                 int             `json:"pc"`
	ME                                 METRICAS_KERNEL `json:"metricas_de_estado"`
	MT                                 METRICAS_KERNEL `json:"metricas_de_tiempo"`
	RutaPseudocodigo                   string          `json:"ruta_pseudocodigo"`
	Tamanio                            int             `json:"tamanio"`
	TiempoInicioEstado                 time.Time       `json:"tiempo_inicio_estado"`
	EstimadoActual                     float32         `json:"estimado_actual"`   // Estimado de tiempo de CPU restante
	EstimadoAnterior                   float32         `json:"estimado_anterior"` // Estimado de tiempo de CPU anterior
	EsperandoFinalizacionDeOtroProceso bool            `json:"esperando_finalizacion_de_otro_proceso"`
}

// Esta estructura las podriamos cambiar por un array de contadores/acumuladores
// Lo cambiamos a metricas kernel para no confundir con las metricas de proceso del modulo de Memoria
type METRICAS_KERNEL struct {
	NEW               int `json:"new"`
	READY             int `json:"ready"`
	RUNNING           int `json:"running"`
	BLOCKED           int `json:"blocked"`
	SUSPENDED_BLOCKED int `json:"suspended_blocked"`
	SUSPENDED_READY   int `json:"suspended_ready"`
	EXIT              int `json:"exit"`
}

type Config struct {
	IP_MEMORY               string  `json:"ip_memory"`
	PORT_MEMORY             int     `json:"port_memory"`
	IP_KERNEL               string  `json:"ip_kernel"`
	PORT_KERNEL             int     `json:"port_kernel"`
	SCHEDULER_ALGORITHM     string  `json:"scheduler_algorithm"`
	READY_INGRESS_ALGORITHM string  `json:"ready_ingress_algorithm"`
	ALPHA                   float32 `json:"alpha"`
	INITIAL_ESTIMATE        float32 `json:"initial_estimate"`
	SUSPENSION_TIME         int     `json:"suspension_time"`
	LOG_LEVEL               string  `json:"log_level"`
}

type ProcesoSuspension struct {
	PID           int
	TiempoBloqueo time.Time
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

type RespuestaIO struct {
	PID                int    `json:"pid"`
	Motivo             string `json:"motivo"`
	Nombre_Dispositivo string `json:"nombre_dispositivo"`
}

type PeticionIO struct {
	PID    int `json:"pid"`
	Tiempo int `json:"tiempo"`
}

type ProcesoEsperandoIO struct {
	PCB    *PCB
	Tiempo int
}

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
var mutexManejandoInterrupcion sync.Mutex // mutex para manejar interrupciones

// Channels

var procesosEnNewOSuspendedReady = make(chan int, 10)
var procesosEnReady = make(chan int, 10)
var procesosEnBlocked = make(chan int, 10)

var cpusDisponibles = make(chan int, 10) // canal para manejar CPUs disponibles

// Conexiones CPU
var ConexionesCPU []globales.HandshakeCPU
var mutexConexionesCPU sync.Mutex

// var mutexInterrupcionDesalojo []sync.Mutex

// Colas de los procesos
var ColaNew *[]*PCB
var ColaReady *[]*PCB
var ColaRunning *[]*PCB
var ColaBlocked *[]*PCB
var ColaSuspendedBlocked *[]*PCB
var ColaSuspendedReady *[]*PCB
var ColaExit *[]*PCB

var PlanificadorActivo bool = false

var UltimoPID int = 0

var ProcesosBlocked []ProcesoSuspension

var algoritmoColaNew string
var algoritmoColaReady string
var alfa float32
var estimadoInicial float32

// lista de ios q se conectaron
var DispositivosIO []*DispositivoIO
var mutexDispositivosIO sync.Mutex // mutex para proteger el acceso a DispositivosIO

var CPUporProceso = make(map[string]int) // clave: ID de CPU, valor: PID del proceso que está ejecutando
var mutexCPUporProceso sync.Mutex        // mutex para proteger el acceso a CPUporProceso

var manejandoInterrupcion bool = false // variable para saber si se está manejando una interrupción

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
	ColaNew = &[]*PCB{}
	ColaReady = &[]*PCB{}
	ColaRunning = &[]*PCB{}
	ColaBlocked = &[]*PCB{}
	ColaSuspendedBlocked = &[]*PCB{}
	ColaSuspendedReady = &[]*PCB{}
	ColaExit = &[]*PCB{}
}

func AgregarPCBaCola(pcb *PCB, cola *[]*PCB) {
	mutex, err := mutexCorrespondiente(cola)
	if err != nil {
		return
	}
	slog.Info(fmt.Sprintf("Antes del lock de cola: %s", obtenerEstadoDeCola(cola)))
	mutex.Lock()
	slog.Info(fmt.Sprintf("Despues del lock de cola: %s", obtenerEstadoDeCola(cola)))

	pcb.TiempoInicioEstado = time.Now()

	// Verificar si el PCB ya está en la cola
	for _, p := range *cola {
		if p.PID == pcb.PID {
			slog.Error(fmt.Sprintf("Intento de agregar PCB %d a %s pero ya está en la cola", pcb.PID, obtenerEstadoDeCola(cola)))
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

	slog.Debug("Antes del switch de channels:")
	switch cola {
	case ColaNew:
		slog.Debug("Entro a cola new")
		procesosEnNewOSuspendedReady <- 1
	case ColaReady:
		slog.Debug("Entro a cola ready")
		procesosEnReady <- 1
	case ColaSuspendedReady:
		slog.Debug("Entro a cola suspended ready")
		procesosEnNewOSuspendedReady <- 1
	case ColaBlocked:
		slog.Debug("Entro a cola blocked")
		procesosEnBlocked <- 1
	}

	slog.Info(fmt.Sprintf("## (%d) agregado a la cola: %s", pcb.PID, obtenerEstadoDeCola(cola)))
	actualizarMetricasEstado(pcb, obtenerEstadoDeCola(cola))
}

func mutexCorrespondiente(cola *[]*PCB) (*sync.Mutex, error) {
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

func LeerPCBDesdeCola(cola *[]*PCB) (*PCB, error) {
	mutex, err := mutexCorrespondiente(cola)
	if err != nil {
		return &PCB{}, fmt.Errorf("no existe mutex correspondiente")
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
		return &PCB{}, fmt.Errorf("no hay PCBs en la cola")
	}
}

func ReinsertarEnFrenteCola(cola *[]*PCB, pcb *PCB) {
	slicePCB := []*PCB{pcb}
	mutex, err := mutexCorrespondiente(cola)
	mutex.Lock()
	if err != nil {
		mutex.Unlock()
		return
	}
	*cola = append(slicePCB, *cola...)
	mutex.Unlock()

	slog.Debug("Antes del switch de channels:")
	switch cola {
	case ColaNew:
		slog.Debug("Entro a cola new")
		procesosEnNewOSuspendedReady <- 1
	case ColaReady:
		slog.Debug("Entro a cola ready")
		procesosEnReady <- 1
	case ColaSuspendedReady:
		slog.Debug("Entro a cola suspended ready")
		procesosEnNewOSuspendedReady <- 1
	case ColaBlocked:
		slog.Debug("Entro a cola blocked")
		procesosEnBlocked <- 1
	}
}

func obtenerEstadoDeCola(cola *[]*PCB) string {
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

func actualizarMetricasTiempo(pcb *PCB, estado string, tiempoMS int64) {
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

func actualizarMetricasEstado(pcb *PCB, estado string) {
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

func buscarPCBYSacarDeCola(pid int, cola *[]*PCB) (*PCB, error) {

	mutex, err := mutexCorrespondiente(cola)
	if err != nil {
		slog.Info("No existe la cola solicitada")
		return &PCB{}, fmt.Errorf("no se ha encontrado el PCB")
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

			return pcb, nil
		}
	}
	mutex.Unlock()

	slog.Info(fmt.Sprintf("No se encontró el PCB del PID %d en la cola %s", pid, obtenerEstadoDeCola(cola)))
	return &PCB{}, fmt.Errorf("no se ha encontrado el PCB")
}

func RecibirProcesoInterrumpido(w http.ResponseWriter, r *http.Request) {
	paquete := globales.Interrupcion{}
	paquete = servidor.DecodificarPaquete(w, r, &paquete)

	pid := paquete.PID
	id_cpu, err := buscarCPUConPid(pid)
	if err != nil {
		slog.Error(fmt.Sprintf("No se encontró la CPU con PID %d", pid))
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("CPU no encontrada"))
		return
	}

	pcb, err := buscarPCBYSacarDeCola(paquete.PID, ColaRunning) // Saco el proceso de la cola de Running
	if err != nil {
		slog.Error(fmt.Sprintf("No se encontró el PCB del PID %d en la cola", paquete.PID))
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Pcb no encontrado"))
	}

	mutexCPUporProceso.Lock()
	delete(CPUporProceso, id_cpu)
	mutexCPUporProceso.Unlock()

	<-cpusDisponibles

	pcb.PC = paquete.PC
	AgregarPCBaCola(pcb, ColaReady)

	mutexManejandoInterrupcion.Lock()
	manejandoInterrupcion = false
	mutexManejandoInterrupcion.Unlock()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func buscarCPUConId(id string) (globales.HandshakeCPU, error) {
	mutexConexionesCPU.Lock()
	for _, cpu := range ConexionesCPU {
		if cpu.ID_CPU == id {
			mutexConexionesCPU.Unlock()
			return cpu, nil
		}
	}
	mutexConexionesCPU.Unlock()

	return globales.HandshakeCPU{}, fmt.Errorf("no se encontró la CPU con ID %s", id)
}

func buscarCPUConPid(pidBuscado int) (string, error) { // devuelve ID de CPU donde estaba proceso con tal PID
	mutexCPUporProceso.Lock()
	for idcpu, pid := range CPUporProceso {
		if pidBuscado == pid {
			mutexCPUporProceso.Unlock()
			return idcpu, nil
		}
	}

	mutexCPUporProceso.Unlock()

	return "", fmt.Errorf("no se encontró la CPU con PID %d", pidBuscado)
}

func AtenderHandshakeCPU(w http.ResponseWriter, r *http.Request) {
	var paquete globales.HandshakeCPU = servidor.DecodificarPaquete(w, r, &globales.HandshakeCPU{})
	slog.Info("Recibido handshake CPU.")

	mutexConexionesCPU.Lock() // bloquea
	for i, cpu := range ConexionesCPU {
		if cpu.ID_CPU == paquete.ID_CPU {
			ConexionesCPU = append(ConexionesCPU[:i], ConexionesCPU[i+1:]...)
			break
		}
	}
	ConexionesCPU = append(ConexionesCPU, paquete)
	mutexConexionesCPU.Unlock() // desbloquea

	cpusDisponibles <- 1 // agrega una CPU disponible al canal

	slog.Debug(fmt.Sprintf("Conexiones CPU: %v", ConexionesCPU))

	//log.Printf("%+v\n", paquete) imprime el handshake del cpu
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func DesconectarCPU(w http.ResponseWriter, r *http.Request) {
	paquete := globales.HandshakeCPU{}
	paquete = servidor.DecodificarPaquete(w, r, &paquete)

	slog.Info(fmt.Sprintf("Desconectando CPU con ID: %s", paquete.ID_CPU))

	mutexConexionesCPU.Lock()
	for i, cpu := range ConexionesCPU {
		if cpu.ID_CPU == paquete.ID_CPU {
			ConexionesCPU = append(ConexionesCPU[:i], ConexionesCPU[i+1:]...)
			mutexCPUporProceso.Lock()
			delete(CPUporProceso, paquete.ID_CPU)
			mutexCPUporProceso.Unlock()
			break
		}
	}
	mutexConexionesCPU.Unlock()
	<-cpusDisponibles

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

	idcpu, err := buscarCPUConPid(pid.NUMERO_PID)
	if err != nil {
		slog.Error(fmt.Sprintf("No se encontró la CPU con PID %d", pid.NUMERO_PID))
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("CPU no encontrada"))
		return
	}
	mutexCPUporProceso.Lock()
	slog.Info(fmt.Sprintf("## CPU por proceso antes del delete: %v", CPUporProceso))
	delete(CPUporProceso, idcpu)
	slog.Info(fmt.Sprintf("## CPU por proceso despues del delete: %v", CPUporProceso))
	mutexCPUporProceso.Unlock()

	cpusDisponibles <- 1 // libera una CPU disponible
	slog.Debug(fmt.Sprintf("Channel: %v", cpusDisponibles))

	//planificadorCortoPlazo.Unlock()
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))

}

func FinalizarProceso(pid int, cola *[]*PCB) {
	slog.Debug(fmt.Sprintf("Cola READY (finalizar proceso): %v \n", &ColaReady))
	slog.Debug(fmt.Sprintf("Cola RUNNING (finalizar proceso): %v \n", &ColaRunning))
	slog.Debug(fmt.Sprintf("Cola EXIT (finalizar proceso): %v \n", &ColaExit))

	slog.Debug(fmt.Sprintf("Finalizando proceso (finalizar proceso) con PID: %d", pid))
	pcb, err := buscarPCBYSacarDeCola(pid, cola)
	if err != nil {
		slog.Error(fmt.Sprintf("No se encontró el PCB del PID %d en la cola", pid))
	} else {

		// conexion con memoria para liberar espacio del PCB
		pid_a_eliminar := globales.DestruirProceso{
			PID: pid,
		}

		// peticion a memoria para liberar el espacio
		slog.Info(fmt.Sprintf("Eliminando proceso con PID: %d de memoria", pid))
		globales.GenerarYEnviarPaquete(&pid_a_eliminar, ClientConfig.IP_MEMORY, ClientConfig.PORT_MEMORY, "/kernel/finalizar_proceso")
		slog.Info(fmt.Sprintf("Se elimino proceso con PID: %d de memoria", pid))
		AgregarPCBaCola(pcb, ColaExit)

		// cambio de estado a Exit del PCB
		slog.Info(fmt.Sprintf("## (%d) - Finaliza el proceso \n", pid)) // log obligatorio de Fin proceso

		actualizarEsperandoFinalizacion(ColaSuspendedReady)
		actualizarEsperandoFinalizacion(ColaNew)

		ImprimirMetricasProceso(*pcb)
	}
}

// Una vez finalizado un proceso, le avisamos a los que no tenían espacio en memoria que pueden intentar entrar
func actualizarEsperandoFinalizacion(cola *[]*PCB) {
	for _, pcb := range *cola {
		if pcb.EsperandoFinalizacionDeOtroProceso {
			pcb.EsperandoFinalizacionDeOtroProceso = false
		}
	}
}

func DumpearMemoria(w http.ResponseWriter, r *http.Request) {
	paquete := globales.SolicitudDump{}
	paquete = servidor.DecodificarPaquete(w, r, &paquete)

	pidABloquear := paquete.PID
	pc := paquete.PC

	id_cpu, err := buscarCPUConPid(pidABloquear)
	if err != nil {
		slog.Error(fmt.Sprintf("No se encontró la CPU con PID %d", pidABloquear))
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("CPU no encontrada"))
		return
	}

	pcbABloquear, err := buscarPCBYSacarDeCola(pidABloquear, ColaRunning)
	if err != nil {
		slog.Error(fmt.Sprintf("No se encontró el PCB del PID %d en la cola", pidABloquear))
	}
	mutexCPUporProceso.Lock()
	delete(CPUporProceso, id_cpu)
	mutexCPUporProceso.Unlock()

	pcbABloquear.PC = pc
	recalcularEstimados(pcbABloquear) // recalculo el estimado del pcb
	AgregarPCBaCola(pcbABloquear, ColaBlocked)
	slog.Info(fmt.Sprintf("## (%d) Pasa del estado RUNNING al estado BLOCKED", pidABloquear))

	peticion := globales.PID{
		NUMERO_PID: pidABloquear,
	}

	var peticionEnviada, _ = globales.GenerarYEnviarPaquete(&peticion, ClientConfig.IP_MEMORY, ClientConfig.PORT_MEMORY, "/kernel/dump_de_proceso")

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

	slog.Info(fmt.Sprintf("## (%d) Se crea el proceso - Estado: NEW", pid))

	pcb := PCB{
		PID:                                pid,
		PC:                                 0,
		RutaPseudocodigo:                   rutaPseudocodigo,
		Tamanio:                            tamanio,
		TiempoInicioEstado:                 time.Now(),
		ME:                                 METRICAS_KERNEL{}, // por defecto go inicializa todo en 0
		MT:                                 METRICAS_KERNEL{},
		EstimadoAnterior:                   estimadoInicial,
		EstimadoActual:                     estimadoInicial,
		EsperandoFinalizacionDeOtroProceso: false,
	}

	AgregarPCBaCola(&pcb, ColaNew)
	ordenarColaNew()
}

func CrearProcesoEnMemoria(pcb *PCB) bool {

	archivoProceso := globales.MEMORIA_CREACION_PROCESO{ // Ida y vuelta con memoria
		PID:                     pcb.PID,
		RutaArchivoPseudocodigo: pcb.RutaPseudocodigo,
		Tamanio:                 pcb.Tamanio,
	}

	resp, _ := globales.GenerarYEnviarPaquete(&archivoProceso, ClientConfig.IP_MEMORY, ClientConfig.PORT_MEMORY, "/kernel/inicializar_proceso")

	// espera 1 segundo para simular la creación del proceso
	if resp.StatusCode == http.StatusOK {
		slog.Debug(fmt.Sprintf("Proceso con PID %d creado en memoria", pcb.PID))
		return true
	} else {
		pcb.EsperandoFinalizacionDeOtroProceso = true // si no se pudo crear, queda esperando a que finalice otro proceso
		slog.Debug(fmt.Sprintf("Error al crear el proceso con PID %d en memoria", pcb.PID))
		return false
	}

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
		slog.Debug("Ordenando cola SUSPENDED_READY por FIFO")
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

	if algoritmoColaReady == "FIFO" {
		slog.Debug("Ordenando cola READY por FIFO")
	} else {
		sort.Slice(*ColaReady, func(i, j int) bool {
			return (*ColaReady)[i].EstimadoActual < (*ColaReady)[j].EstimadoActual
		})
	}
	/*
		for _, p := range *ColaReady {
			slog.Debug(fmt.Sprintf("Pid: (%d) , %d \n", p.PID, p.Tamanio))
		}
	*/
	mutexColaReady.Unlock()

}

func recalcularEstimados(pcb *PCB) {
	pcb.EstimadoAnterior = pcb.EstimadoActual
	ultimaRafaga := time.Since(pcb.TiempoInicioEstado).Milliseconds()

	pcb.EstimadoActual = (float32(ultimaRafaga) * alfa) + (pcb.EstimadoAnterior)*(1-alfa)
}

// envia peticion a memoria para q mueva un proceso a swap
func EnviarProcesoASwap(pcb PCB) bool {
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

// inicia todos los planificadores
func IniciarPlanificadores() {
	PlanificadorActivo = true

	go PlanificadorLargoPlazo()
	go PlanificadorMedianoPlazo()
	go PlanificadorCortoPlazo()

	slog.Info("Planificadores iniciados: largo, corto y mediano plazo")
}

// atiende la cola de new y pasa a ready si hay espacio
func PlanificadorLargoPlazo() {
	for PlanificadorActivo {
		<-procesosEnNewOSuspendedReady // espera a que haya un proceso en new o suspended ready
		// primero miro suspendidos ready, que tiene prioridad por sobre ready
		atenderColaSuspendidosReady()

		if len(*ColaSuspendedReady) > 0 {
			continue
		}

		// si no hay procesos en new, sigo
		if len(*ColaNew) == 0 {
			continue
		}

		pcb := (*ColaNew)[0]

		if pcb.EsperandoFinalizacionDeOtroProceso {
			slog.Debug(fmt.Sprintf("## (%d) Proceso en NEW esperando finalización de otro proceso", (*ColaNew)[0].PID))

			continue
		}

		pcb, err := LeerPCBDesdeCola(ColaNew)
		if err != nil {

			continue
		}

		inicializado := CrearProcesoEnMemoria(pcb) // intento crear el proceso en memoria

		if inicializado {
			// Si hay respuesta positiva de memoria, pasa a ready
			AgregarPCBaCola(pcb, ColaReady)
			ordenarColaReady()
			slog.Info(fmt.Sprintf("## (%d) Pasa del estado NEW al estado READY", pcb.PID))
		} else {
			// No hay respuesta positiva de memoria, queda en new
			AgregarPCBaCola(pcb, ColaNew)
			ordenarColaNew()
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

	if pcb.EsperandoFinalizacionDeOtroProceso {
		slog.Debug(fmt.Sprintf("## (%d) Proceso en SUSPENDED_READY esperando finalización de otro proceso", pcb.PID))
		AgregarPCBaCola(pcb, ColaSuspendedReady) // lo vuelvo a poner en suspended ready
		ordenarColaSuspendedReady()
		return
	}
	// siempre true por ahora
	inicializado := true //desuspender proceso

	if inicializado {
		AgregarPCBaCola(pcb, ColaReady) // lo paso a ready
		ordenarColaReady()
		slog.Info(fmt.Sprintf("## (%d) Pasa del estado SUSPENDED_READY al estado READY", pcb.PID))
	} else {
		// no se pudo, vuelve a cola
		AgregarPCBaCola(pcb, ColaSuspendedReady)
		ordenarColaSuspendedReady()
	}
}

// planificador de corto plazo fifo
func PlanificadorCortoPlazo() {
	slog.Info("antes del for corto plazo")
	if PlanificadorActivo {
		for {
			slog.Debug("HAY UN PROCESO EN READY")
			<-procesosEnReady // espera a que haya un proceso en ready
			switch algoritmoColaReady {
			case "FIFO":
				slog.Debug("Antes de FIFO")
				planificarSinDesalojo()
			case "SJF":
				planificarSinDesalojo() // sin desalojo
			case "SRT":
				slog.Debug("Antes de SRT")
				planificarConDesalojo() // con desalojo
			default:
				planificarSinDesalojo()
			}
		}
	}
}

func BuscarCPULibre() (globales.HandshakeCPU, error) {
	// Lock both mutexes in a consistent order, copy the data, unlock, then process
	mutexConexionesCPU.Lock()
	cpus := make([]globales.HandshakeCPU, len(ConexionesCPU))
	copy(cpus, ConexionesCPU)
	mutexConexionesCPU.Unlock()

	mutexCPUporProceso.Lock()
	cpusOcupadas := make(map[string]int, len(CPUporProceso))
	for k, v := range CPUporProceso {
		cpusOcupadas[k] = v
	}
	mutexCPUporProceso.Unlock()

	if len(cpus)-len(cpusOcupadas) <= 0 {
		slog.Debug("No hay CPUs disponibles")
		return globales.HandshakeCPU{}, fmt.Errorf("no hay CPUs disponibles")
	}

	for _, cpu := range cpus {
		if _, ok := cpusOcupadas[cpu.ID_CPU]; !ok {
			slog.Debug(fmt.Sprintf("CPU libre encontrada: %s", cpu.ID_CPU))
			return cpu, nil
		}
	}
	return globales.HandshakeCPU{}, fmt.Errorf("no hay CPUs disponibles")
}

func EnviarProcesoACPU(pcb *PCB, cpu globales.HandshakeCPU) {
	peticionCPU := globales.PeticionCPU{
		PID: pcb.PID,
		PC:  pcb.PC,
	}

	ip := cpu.IP_CPU
	puerto := cpu.PORT_CPU
	slog.Info(fmt.Sprintf("El ID del CPU es %s, Puerto %d", cpu.ID_CPU, puerto))
	url := fmt.Sprintf("/cpu/%s/ejecutarProceso", cpu.ID_CPU)

	slog.Debug("Intentando enviar pcb a cpu ...")

	slog.Info(fmt.Sprintf("## (%d) Pasa del estado READY al estado RUNNING", pcb.PID))
	AgregarPCBaCola(pcb, ColaRunning)
	// no mover esto a abajo del generar y enviar paquete, porque se rompe

	//agrego el proceso al map de procesos por CPU
	mutexCPUporProceso.Lock()
	CPUporProceso[cpu.ID_CPU] = pcb.PID
	mutexCPUporProceso.Unlock()

	resp, _ := globales.GenerarYEnviarPaquete(&peticionCPU, ip, puerto, url)
	if resp.StatusCode != 200 {
		slog.Error(fmt.Sprintf("Error al enviar el proceso a la CPU: %s", resp.Status))
		cpusDisponibles <- 1 // vuelvo a liberar el canal de cpus disponibles
		ReinsertarEnFrenteCola(ColaReady, pcb)
		return
	}
}

func planificarSinDesalojo() {
	planificadorCortoPlazo.Lock()

	slog.Debug(fmt.Sprintf("Conexiones CPU del sistema: %v", ConexionesCPU))

	cpuLibre, err := BuscarCPULibre() // busco una cpu disponible

	<-cpusDisponibles

	if err != nil {
		planificadorCortoPlazo.Unlock()
		slog.Debug("No hay CPUs disponibles")
		cpusDisponibles <- 1 // vuelvo a liberar el canal de cpus disponibles
		return
	}

	pcb, err := LeerPCBDesdeCola(ColaReady)
	if err != nil {
		planificadorCortoPlazo.Unlock()
		slog.Debug("No hay procesos listos") // borrar
		procesosEnReady <- 1                 // vuelvo a liberar el canal de procesos en ready
		return
	}
	slog.Debug(fmt.Sprintf("CPU LIBRE: %v", cpuLibre))

	go EnviarProcesoACPU(pcb, cpuLibre)
	planificadorCortoPlazo.Unlock()
}

func planificarConDesalojo() {
	//time.Sleep(400 * time.Millisecond)
	<-cpusDisponibles
	slog.Debug("Arranca el for de planificador con desalojo")
	cpuLibre, errCPU := BuscarCPULibre()
	slog.Debug("Antes del for de cpusLibres")
	// Mientras haya CPUs libres y procesos en READY, planificar
	planificadorCortoPlazo.Lock()
	if errCPU == nil && len(*ColaReady) > 0 {

		//planificadorCortoPlazo.Lock()
		slog.Info("Paso for, hay cpus libres y procesos en ready")
		pcbReady, errReady := obtenerMenorEstimadoDeReady()
		if errReady != nil {
			planificadorCortoPlazo.Unlock()
			cpusDisponibles <- 1 // vuelvo a liberar el canal de cpus disponibles
			return
		}
		slog.Debug(fmt.Sprintf("PCB ready %v", pcbReady.PID))
		_, err := buscarPCBYSacarDeCola(pcbReady.PID, ColaReady)
		if err != nil {
			planificadorCortoPlazo.Unlock()
			cpusDisponibles <- 1 // vuelvo a liberar el canal de cpus disponibles
			return
		}
		slog.Debug(fmt.Sprintf("Enviando PCB %d a CPU %s", pcbReady.PID, cpuLibre.ID_CPU))
		go EnviarProcesoACPU(pcbReady, cpuLibre)
		planificadorCortoPlazo.Unlock()
		slog.Debug("ANTES DEL CONTINUE")
		return
	}
	cpusDisponibles <- 1 // vuelvo a liberar el canal de cpus disponibles
	slog.Debug("NO HAY CPUS LIBRES")

	// Si no hay CPUs libres, evaluar desalojo
	slog.Debug(fmt.Sprintf("CANTIDAD DE PROCESOS EN READY: %d", len(*ColaReady)))
	slog.Debug(fmt.Sprintf("ERROR BUSCANDO CPU: %s", errCPU))
	if len(*ColaReady) > 0 {
		//planificadorCortoPlazo.Lock()

		slog.Debug("Buscando PCB con menor estimado de READY")
		pcbReady, errReady := obtenerMenorEstimadoDeReady()

		slog.Debug("Buscando PCB con mayor estimado de RUNNING")
		pcbMasLento, errRunning := obtenerMayorEstimadoDeRunning()

		slog.Debug(fmt.Sprintf("Cola RUNNING EN PLANIFICADOR CON DESALOJO: %v", ColaRunning))

		if !manejandoInterrupcion && errReady == nil && errRunning == nil && pcbReady.EstimadoActual < pcbMasLento.EstimadoActual {
			slog.Info(fmt.Sprintf("Intentando desalojar PID %d para ejecutar PID %d", pcbMasLento.PID, pcbReady.PID))
			cpuEjecutando, err2 := buscarCPUConPid(pcbMasLento.PID) // del que esta en running

			if err2 == nil && cpuEjecutando != "" {
				InterrumpirProceso(pcbMasLento, cpuEjecutando)
				planificadorCortoPlazo.Unlock()
				return
			}
			slog.Debug("Salgo del if de desalojo cpu (1)")
			planificadorCortoPlazo.Unlock()
			procesosEnReady <- 1 // vuelvo a liberar el canal de procesos en ready
			return
		}
		slog.Debug("Salgo del if de desalojo cpu (2)")
		planificadorCortoPlazo.Unlock()
		procesosEnReady <- 1 // vuelvo a liberar el canal de procesos en ready
		return
		// planificadorCortoPlazo.Unlock()
	}
	planificadorCortoPlazo.Unlock()
}

func InterrumpirProceso(pcb *PCB, id_cpu string) {

	slog.Info(fmt.Sprintf("Enviando interrupción a CPU %s para desalojar PID %d", id_cpu, pcb.PID))

	cpu, err := buscarCPUConId(id_cpu)
	if err != nil {
		slog.Error(fmt.Sprintf("No se encontró la CPU con ID %s: %v", id_cpu, err))
		return
	}

	interrupcion := globales.Interrupcion{
		PID:    pcb.PID,
		MOTIVO: "Desalojo por SRT",
	}

	endpoint := fmt.Sprintf("/cpu/%s/interruptDesalojo", cpu.ID_CPU)

	resp, _ := globales.GenerarYEnviarPaquete(&interrupcion, cpu.IP_CPU, cpu.PORT_CPU, endpoint)

	if resp.StatusCode != 200 {
		slog.Error(fmt.Sprintf("Error al enviar la interrupción a la CPU %s: %s", cpu.ID_CPU, resp.Status))
		return
	}

	slog.Info(fmt.Sprintf("Interrupción enviada correctamente a CPU %s para PID %d", cpu.ID_CPU, pcb.PID))
	mutexManejandoInterrupcion.Lock()
	manejandoInterrupcion = true
	mutexManejandoInterrupcion.Unlock()
}

// Devuelve el PCB con menor estimado de la cola READY
func obtenerMenorEstimadoDeReady() (*PCB, error) {
	if len(*ColaReady) == 0 {
		return nil, fmt.Errorf("cola READY vacía")
	}
	ordenarColaReady()

	return (*ColaReady)[0], nil
}

// Devuelve el PCB con mayor estimado de la cola RUNNING
func obtenerMayorEstimadoDeRunning() (*PCB, error) {
	mutexColaRunning.Lock()
	if len(*ColaRunning) == 0 {
		mutexColaRunning.Unlock()
		return nil, fmt.Errorf("cola RUNNING vacía")
	}
	max := (*ColaRunning)[0]
	for _, p := range *ColaRunning {
		if p.EstimadoActual > max.EstimadoActual {
			max = p
		}
	}

	mutexColaRunning.Unlock()
	return max, nil
}

// planificador de mediano plazo
func PlanificadorMedianoPlazo() {
	for PlanificadorActivo {
		// reviso cada proceso bloqueado
		for i := 0; i < len(*ColaBlocked); i++ {
			// calculo tiempo bloqueado
			tiempoBloqueo := time.Since((*ColaBlocked)[i].TiempoInicioEstado)

			// si excede el tiempo de suspension lo suspendo
			if tiempoBloqueo.Milliseconds() > int64(ClientConfig.SUSPENSION_TIME) {
				pid := (*ColaBlocked)[i].PID

				pcb, err := buscarPCBYSacarDeCola(pid, ColaBlocked)
				if err != nil {
					ReinsertarEnFrenteCola(ColaBlocked, pcb)
					slog.Error(fmt.Sprintf("No se encontró el PCB del PID %d en la cola", pid))
				}

				if err == nil {
					// informo a memoria q lo mueva a swap
					swapExitoso := EnviarProcesoASwap(*pcb)

					if swapExitoso {
						// lo paso a susp_blocked
						AgregarPCBaCola(pcb, ColaSuspendedBlocked)
						slog.Info(fmt.Sprintf("## (%d) Pasa del estado BLOCKED al estado SUSPENDED_BLOCKED", pcb.PID))

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

func ImprimirMetricasProceso(pcb PCB) {
	slog.Info(fmt.Sprintf("## (%d) - Métricas de estado: NEW (%d) (%d), READY (%d) (%d), RUNNING (%d) (%d), BLOCKED (%d) (%d), SUSPENDED_BLOCKED (%d) (%d), SUSPENDED_READY (%d) (%d), EXIT (%d) (%d)",
		pcb.PID,
		pcb.ME.NEW, pcb.MT.NEW,
		pcb.ME.READY, pcb.MT.READY,
		pcb.ME.RUNNING, pcb.MT.RUNNING,
		pcb.ME.BLOCKED, pcb.MT.BLOCKED,
		pcb.ME.SUSPENDED_BLOCKED, pcb.MT.SUSPENDED_BLOCKED,
		pcb.ME.SUSPENDED_READY, pcb.MT.SUSPENDED_READY,
		pcb.ME.EXIT, pcb.MT.EXIT))
	slog.Info(fmt.Sprintf("\nEstimado Anterior: %f, Estimado Actual: %f",
		pcb.EstimadoAnterior, pcb.EstimadoActual))
}

func mandarProcesoAIO(instancia *InstanciaIO, dispositivoIO *DispositivoIO) {
	slog.Debug(fmt.Sprintf("## Dispositivo IO %s inicio goroutine", dispositivoIO.Nombre))
	cola := &dispositivoIO.Cola
	ipIO := instancia.IP
	puertoIO := instancia.Puerto

	for instancia.EstaConectada {
		if instancia.EstaDisponible {
			<-dispositivoIO.procesosEsperandoIO
			//time.Sleep(1 * time.Second) // espera 1 segundo antes de verificar la cola nuevamente
			slog.Debug(fmt.Sprintf("## Dispositivo IO %s revisando cola %v", dispositivoIO.Nombre, (*cola)))

			if len(*cola) > 0 {

				dispositivoIO.MutexCola.Lock()
				proceso := (*cola)[0]
				(*cola) = (*cola)[1:]
				dispositivoIO.MutexCola.Unlock()
				instancia.EstaDisponible = false
				peticionEnviada := EnviarPeticionIO(proceso.PCB, ipIO, puertoIO, proceso.Tiempo)

				slog.Debug(fmt.Sprintf("## valor de peticion enviada: %t", peticionEnviada))

				if !peticionEnviada {
					slog.Error(fmt.Sprintf("## Error al enviar la peticion de IO al dispositivo %s", dispositivoIO.Nombre))
					FinalizarProceso(proceso.PCB.PID, ColaBlocked)
					instancia.EstaDisponible = true
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
	pidABloquear := paquete.PID

	id_cpu, err := buscarCPUConPid(pidABloquear)
	if err != nil {
		slog.Error(fmt.Sprintf("No se encontró la CPU con PID %d", pidABloquear))
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("CPU no encontrada"))
		return
	}

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
	slog.Info(fmt.Sprintf("## CPU por proceso antes del delete: %v", CPUporProceso))
	mutexCPUporProceso.Lock()
	delete(CPUporProceso, id_cpu)
	mutexCPUporProceso.Unlock() // saco el proceso de la cpu
	slog.Info(fmt.Sprintf("## CPU por proceso despues del delete: %v", CPUporProceso))

	recalcularEstimados(pcbABloquear) // recalculo estimados antes de bloquear
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
	ioDevice.procesosEsperandoIO <- 1

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

type DispositivoIO struct {
	Nombre              string
	Cola                []*ProcesoEsperandoIO // cola de peticiones de IO
	MutexCola           *sync.Mutex           // mutex para garantizar que entre un pcb a la vez a la cola
	TengoInstancias     bool
	Instancias          []*InstanciaIO // lista de instancias del dispositivo IO
	procesosEsperandoIO chan int
}
type InstanciaIO struct {
	IP             string
	Puerto         int
	EstaDisponible bool
	EstaConectada  bool
}

// guarda los IO q se conectan
func AtenderHandshakeIO(w http.ResponseWriter, r *http.Request) {
	paquete := HandshakeIO{}
	paquete = servidor.DecodificarPaquete(w, r, &paquete)

	slog.Info(fmt.Sprintf("Recibido handshake del dispositivo IO: %s", paquete.Nombre))

	mutexDispositivosIO.Lock()
	for _, dispositivo := range DispositivosIO {
		if dispositivo.Nombre == paquete.Nombre {
			instancia := InstanciaIO{
				IP:             paquete.IP,
				Puerto:         paquete.Puerto,
				EstaDisponible: true,
				EstaConectada:  true,
			}

			dispositivo.Instancias = append(dispositivo.Instancias, &instancia)
			mutexDispositivosIO.Unlock()
			slog.Debug(fmt.Sprintf("Dispositivo IO registrado: %+v\n", paquete))

			go mandarProcesoAIO(dispositivo.Instancias[len(dispositivo.Instancias)-1], dispositivo) // mandar goroutine para atender el dispositivo IO
		}
	}
	mutexDispositivosIO.Unlock()

	instancia := InstanciaIO{
		IP:             paquete.IP,
		Puerto:         paquete.Puerto,
		EstaDisponible: true,
		EstaConectada:  true,
	}

	dispositivoIO := DispositivoIO{
		Nombre:              paquete.Nombre,
		Cola:                make([]*ProcesoEsperandoIO, 0),
		MutexCola:           new(sync.Mutex),
		TengoInstancias:     true,
		Instancias:          make([]*InstanciaIO, 0),
		procesosEsperandoIO: make(chan int, 10),
	}
	dispositivoIO.Instancias = append(dispositivoIO.Instancias, &instancia)

	mutexDispositivosIO.Lock()
	DispositivosIO = append(DispositivosIO, &dispositivoIO)
	mutexDispositivosIO.Unlock()

	log.Printf("Dispositivo IO registrado: %+v\n", paquete)

	go mandarProcesoAIO(&instancia, &dispositivoIO)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// envia peticion al dispositivo io disponible
func EnviarPeticionIO(pcbABloquear *PCB, ipIO string, puertoIO int, tiempoIO int) bool {

	peticion := PeticionIO{ // armo el paquete con pid y tiempo
		PID:    pcbABloquear.PID,
		Tiempo: tiempoIO,
	}

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
		ordenarColaSuspendedReady()
		slog.Info(fmt.Sprintf("## (%d) Pasa del estado SUSPENDED_BLOCKED al estado SUSPENDED_READY", pcb.PID))

		// Deswapear proceso
		/*
			//mutexProcesosBloqueados.Lock()
			for i, proceso := range ProcesosBlocked {
				if proceso.PID == pidFinIO {
					ProcesosBlocked = append(ProcesosBlocked[:i], ProcesosBlocked[i+1:]...)
					mutexProcesosBloqueados.Unlock()
					slog.Info(fmt.Sprintf("## (%d) - Eliminado de la lista de procesos bloqueados", pidFinIO))
					break
				}
			}
		*/
		//mutexProcesosBloqueados.Unlock()

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))

		return
	} else {
		pcb, err := buscarPCBYSacarDeCola(pidFinIO, ColaBlocked)
		if err == nil {
			AgregarPCBaCola(pcb, ColaReady)
			ordenarColaReady()
			slog.Info(fmt.Sprintf("## (%d) Pasa del estado BLOCKED al estado READY", pcb.PID))
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
			return
		} else {
			slog.Error(fmt.Sprintf("No se encontró el PCB del PID %d en las colas blocked/suspended_Blocked", pidFinIO))
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("pcb no encontrado"))
			return
		}
	}

}

func DesconectarIO(w http.ResponseWriter, r *http.Request) {
	paquete := HandshakeIO{}
	paquete = servidor.DecodificarPaquete(w, r, &paquete)

	mutexDispositivosIO.Lock()
	for _, dispositivo := range DispositivosIO {
		if dispositivo.Nombre == paquete.Nombre && dispositivo.IP == paquete.IP && dispositivo.Puerto == paquete.Puerto {
			slog.Info(fmt.Sprintf("Desconectando dispositivo IO: %s", dispositivo.Nombre))
			dispositivo.EstaConectado = false // Marcar como desconectado
			break
		}
	}
	mutexDispositivosIO.Unlock()

}
