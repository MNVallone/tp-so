package utils

import (
	"encoding/json"
	"fmt"
	"globales"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
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
	EstaEnSwap                         chan int
	RafagaAnterior                     float32 `json:"rafaga_anterior"` // Rafaga anterior del proceso
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
	EstaDisponible chan int
	EstaConectada  bool
}

type RespuestaIO struct {
	PID                int    `json:"pid"`
	Motivo             string `json:"motivo"`
	Nombre_Dispositivo string `json:"nombre_dispositivo"`
	IP                 string `json:"ip"`
	Puerto             int    `json:"puerto"`
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
var RutaConfig string

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
var mutexOrdenandoColaReady sync.Mutex
var mutexProcesosEsperandoAFinalizar sync.Mutex

// Channels
var ProcesosEnNew = make(chan int, 500)
var ProcesosEnSuspendedReady = make(chan int, 50)
var ProcesosEnReady = make(chan int, 50)
var ProcesosEnBlocked = make(chan int, 40)
var ProcesosAFinalizar = make(chan int, 10) // canal para recibir procesos a finalizar
// var ProcesoLlegaAReady = make (chan int, 15)

var CpusDisponibles = make(chan int, 8) // canal para manejar CPUs disponibles
//var Planificando = make(chan int, 1)    // canal para manejar la planificación de procesos
//var EsperandoInterrupcion = make(chan int, 1)

// Conexiones CPU
var ConexionesCPU []globales.HandshakeCPU
var mutexConexionesCPU sync.Mutex
var InterrumpirCPU = make(chan int, 1)

// Colas de los procesos
var ColaNew *[]*PCB
var ColaReady *[]*PCB
var ColaRunning *[]*PCB
var ColaBlocked *[]*PCB
var ColaSuspendedBlocked *[]*PCB
var ColaSuspendedReady *[]*PCB
var ColaExit *[]*PCB

var ProcesosSiendoSwapeados *[]*PCB
var ProcesosEsperandoAFinalizar *[]*PCB

var PlanificadorActivo bool = false

var UltimoPID int = 0

var algoritmoColaNew string
var algoritmoColaReady string
var alfa float32
var estimadoInicial float32

// lista de ios q se conectaron
var DispositivosIO []*DispositivoIO
var mutexDispositivosIO sync.Mutex // mutex para proteger el acceso a DispositivosIO

var CPUporProceso = make(map[string]int) // clave: ID de CPU, valor: PID del proceso que está ejecutando
var mutexCPUporProceso sync.Mutex        // mutex para proteger el acceso a CPUporProceso

var cpupendienteInterrupcion = make(map[string]bool)
var mutexInterrupcionesCPU sync.Mutex

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

	slog.Debug(fmt.Sprintf("Configuración cargada: %+v", *config))

	algoritmoColaNew = config.READY_INGRESS_ALGORITHM
	algoritmoColaReady = config.SCHEDULER_ALGORITHM
	alfa = config.ALPHA
	estimadoInicial = config.INITIAL_ESTIMATE

	/* if algoritmoColaReady == "SRT" {
		go interrumpirCpu()
	} */

	return config
}

func ValidarArgumentosKernel() (string, int) {
	if len(os.Args) < 4 {
		fmt.Println("Error: Falta el archivo de config")
		fmt.Println("Uso: ./kernel [archivo_pseudocodigo] [tamanio_proceso] [archivo_config]")
		os.Exit(1)
	}

	if len(os.Args) < 2 {
		fmt.Println("Error: Falta el archivo de pseudocódigo")
		fmt.Println("Uso: ./kernel [archivo_pseudocodigo] [tamanio_proceso] [archivo_config]")
		os.Exit(1)
	}

	if len(os.Args) < 3 {
		fmt.Println("Error: Falta el tamaño del proceso")
		fmt.Println("Uso: ./kernel [archivo_pseudocodigo] [tamanio_proceso] [archivo_config]")
		os.Exit(1)
	}

	rutaInicial := os.Args[1]
	tamanio, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Println("Error: El tamaño del proceso debe ser un número entero")
		os.Exit(1)
	}

	dir, _ := filepath.Abs(".")

	/* 	// Obtiene la ruta del directorio padre
	   	parentDir := filepath.Dir(dir) */

	RutaConfig = filepath.Join(dir, "configs", os.Args[3])

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
	ProcesosSiendoSwapeados = &[]*PCB{}
	ProcesosEsperandoAFinalizar = &[]*PCB{}
	//Planificando <- 1
}

func AgregarPCBaCola(pcb *PCB, cola *[]*PCB) {
	mutex, err := mutexCorrespondiente(cola)
	if err != nil {
		return
	}

	slog.Debug(fmt.Sprintf("Antes del lock de cola: %s", obtenerEstadoDeCola(cola)))

	pcb.TiempoInicioEstado = time.Now()

	// Verificar si el PCB ya está en la cola
	mutex.Lock()
	defer mutex.Unlock()
	for _, p := range *cola {
		if p.PID == pcb.PID {
			slog.Error(fmt.Sprintf("Intento de agregar PCB %d a %s pero ya está en la cola", pcb.PID, obtenerEstadoDeCola(cola)))
			//mutex.Unlock()
			return
		}
	}

	// Verificar que el PCB no esté en estado EXIT
	if obtenerEstadoDeCola(cola) == "READY" && pcb.ME.EXIT > 0 {
		slog.Error(fmt.Sprintf("Intento de agregar PCB con PID %d a READY, pero ya está en EXIT", pcb.PID))
		//mutex.Unlock()
		return
	}

	*cola = append(*cola, pcb)
	//mutex.Unlock()
	slog.Debug(fmt.Sprintf("Despues del lock de cola: %s", obtenerEstadoDeCola(cola)))

	slog.Debug("Antes del switch de channels:")
	switch cola {
	case ColaNew:
		slog.Debug("Entro a cola new")
		ProcesosEnNew <- 1
		slog.Debug(fmt.Sprintf("Valor channel procesos en new (AgregarPCBACola + 1): %d", len(ProcesosEnNew)))
		/* 		if len(ProcesosEnNew) < cap(ProcesosEnNew) {
			PlanificadorDeLargoPlazo <- 1
			slog.Warn(fmt.Sprintf("Valor channel procesos en compartido (AgregarPCBACola + 1): %d", len(PlanificadorDeLargoPlazo)))
		} */
	/*
		case ColaReady:
			slog.Debug("Entro a cola ready")
			//ProcesosEnReady <- 1

			slog.Debug("valor channel procesos en ready (agregarPCBACola + 1)")*/
	//slog.Info(fmt.Sprintf("Valor channel procesos en ready (AgregarPCBACola): %d", len(ProcesosEnReady)))
	case ColaSuspendedReady:
		slog.Debug("Entro a cola suspended ready")
		ProcesosEnSuspendedReady <- 1
		slog.Debug(fmt.Sprintf("Valor channel procesos en suspended ready (AgregarPCBACola + 1): %d", len(ProcesosEnSuspendedReady)))

		/* 		if len(ProcesosEnSuspendedReady) < cap(ProcesosEnSuspendedReady) {
			PlanificadorDeLargoPlazo <- 1
			slog.Warn(fmt.Sprintf("Valor channel compartido (AgregarPCBACola + 1): %d", len(PlanificadorDeLargoPlazo)))

		} */

		//slog.Debug(fmt.Sprintf("Valor channel procesos en suspened ready (AgregarPCBACola + 1): %d", len(ProcesosEnSuspendedReady)))

	case ColaBlocked: //
		slog.Debug("Entro a cola blocked")
		//ProcesosEnBlocked <- 1
		slog.Debug(fmt.Sprintf("Valor del canal de procesos en blocked: %d", len(ProcesosEnBlocked)))
	}

	slog.Debug(fmt.Sprintf("## (%d) agregado a la cola: %s", pcb.PID, obtenerEstadoDeCola(cola)))
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
	mutex.Lock()
	defer mutex.Unlock()
	if len(*cola) > 0 {
		pcb := (*cola)[0]
		*cola = (*cola)[1:]
		//mutex.Unlock()

		tiempoTranscurrido := time.Since(pcb.TiempoInicioEstado).Milliseconds()
		if cola == ColaRunning {
			pcb.RafagaAnterior += float32(tiempoTranscurrido)
		}

		actualizarMetricasTiempo(pcb, obtenerEstadoDeCola(cola), tiempoTranscurrido)

		slog.Debug(fmt.Sprintf("PCB leido desde la cola: %v", pcb))
		return pcb, nil
	} else {
		//mutex.Unlock()
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
		ProcesosEnNew <- 1
		slog.Debug(fmt.Sprintf("Valor channel procesos en new (ReinsertarEnFrenteCola + 1): %d", len(ProcesosEnNew)))

	/*case ColaReady:
	slog.Debug("Entro a cola ready")
	ProcesosEnReady <- 1*/

	case ColaSuspendedReady:
		slog.Debug("Entro a cola suspended ready")
		ProcesosEnSuspendedReady <- 1
		slog.Debug(fmt.Sprintf("Valor channel procesos en suspened ready (ReinsertarEnFrenteCola + 1): %d", len(ProcesosEnSuspendedReady)))

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

func BuscarColaPorPID(pid int) *[]*PCB {
	colas := []*[]*PCB{ColaNew, ColaReady, ColaRunning, ColaBlocked, ColaSuspendedBlocked, ColaSuspendedReady, ColaExit}
	for _, cola := range colas {
		for _, pcb := range *cola {
			if pcb.PID == pid {
				return cola
			}
		}
	}
	return nil
}

func actualizarMetricasTiempo(pcb *PCB, estado string, tiempoMS int64) {
	slog.Debug(fmt.Sprintf("Actualizando métricas de tiempo para el PCB %d en estado %s con tiempo %d ms", pcb.PID, estado, tiempoMS))
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
		slog.Debug("No existe la cola solicitada")
		return &PCB{}, fmt.Errorf("no se ha encontrado el PCB")
	}
	mutex.Lock()
	for i, p := range *cola {
		if p.PID == pid {

			pcb := p

			// Actualizar el tiempo transcurrido en el estado anterior
			tiempoTranscurrido := time.Since(pcb.TiempoInicioEstado).Milliseconds()
			slog.Debug(fmt.Sprintf("## PID (%d) Tiempo transcurrido en %s: %d", pcb.PID, obtenerEstadoDeCola(cola), tiempoTranscurrido))
			if cola == ColaRunning {
				pcb.RafagaAnterior += float32(tiempoTranscurrido)
				slog.Debug(fmt.Sprintf("Aumenta rafaga anterior de PID: %d, Rafaga Anterior: %f", pcb.PID, pcb.RafagaAnterior))
			}
			actualizarMetricasTiempo(pcb, obtenerEstadoDeCola(cola), tiempoTranscurrido)

			// lo saco de bloqueados
			*cola = append((*cola)[:i], (*cola)[i+1:]...)
			mutex.Unlock()

			return pcb, nil
		}
	}
	mutex.Unlock()

	slog.Debug(fmt.Sprintf("No se encontró el PCB del PID %d en la cola %s", pid, obtenerEstadoDeCola(cola)))
	return &PCB{}, fmt.Errorf("no se ha encontrado el PCB")
}

func RecibirProcesoInterrumpido(w http.ResponseWriter, r *http.Request) {
	paquete := globales.Interrupcion{}
	paquete = globales.DecodificarPaquete(w, r, &paquete)

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

		slog.Debug("Antes del mutexInterrupcionesCPU")
		mutexInterrupcionesCPU.Lock()
		slog.Debug("Despues del mutexInterrupcionesCPU")
		delete(cpupendienteInterrupcion, id_cpu)
		mutexInterrupcionesCPU.Unlock()
		select {
		case <-InterrumpirCPU:
			slog.Debug("Señal de InterrumpirCPU consumida")
		default:
			slog.Debug("No había señal pendiente en InterrumpirCPU")
		}
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Pcb no encontrado"))
		return
	}
	slog.Debug("antes de borrar la interrupcion del map")

	mutexInterrupcionesCPU.Lock()
	delete(cpupendienteInterrupcion, id_cpu)
	mutexInterrupcionesCPU.Unlock()
	slog.Debug("Antes del mutexOrdenandoColaReady")
	pcb.PC = paquete.PC
	mutexOrdenandoColaReady.Lock()
	slog.Debug("Despues del mutexOrdenandoColaReady")
	AgregarPCBaCola(pcb, ColaReady)

	ordenarColaReady()
	mutexOrdenandoColaReady.Unlock()
	ProcesosEnReady <- 1
	//EsperandoInterrupcion <- 1
	select {
	case <-InterrumpirCPU:
		slog.Debug("Señal de InterrumpirCPU consumida")
	default:
		slog.Debug("No había señal pendiente en InterrumpirCPU")
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func buscarCPUConId(id string) (*globales.HandshakeCPU, error) {
	mutexConexionesCPU.Lock()
	for i := range ConexionesCPU {
		if ConexionesCPU[i].ID_CPU == id {
			mutexConexionesCPU.Unlock()
			return &ConexionesCPU[i], nil
		}
	}
	mutexConexionesCPU.Unlock()

	return nil, fmt.Errorf("no se encontró la CPU con ID %s", id)
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
	var paquete globales.HandshakeCPU = globales.DecodificarPaquete(w, r, &globales.HandshakeCPU{})
	slog.Debug("Recibido handshake CPU.")
	//mutexCPUporProceso.Lock()
	if paquete.ID_CPU == "" {
		slog.Error("Handshake CPU recibido sin ID_CPU")
		//mutexCPUporProceso.Unlock()
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("ID_CPU no puede estar vacío"))
		return
	}
	//mutexCPUporProceso.Unlock()

	//slog.Warn("(antes) MutexConexionesCPU")
	mutexConexionesCPU.Lock() // bloquea
	loConozco, cpu := conozcoCPU(paquete.ID_CPU)
	if loConozco {
		mutexConexionesCPU.Unlock() // desbloquea
		//slog.Warn("(despues) MutexConexionesCPU")
		//slog.Info("conozco cpu, agrego valor al channel")
	} else {
		paquete.DISPONIBLE = make(chan int, 1)
		ConexionesCPU = append(ConexionesCPU, paquete)
		paquete.CONECTADA = true // marca la CPU como conectada
		//cpu.CONECTADA = true
		mutexConexionesCPU.Unlock() // desbloquea
		//slog.Warn("(despues) MutexConexionesCPU")

		//slog.Warn(fmt.Sprintf("Antes de escribir en paquete.DISPONIBLE con ID: %s, len: %d", paquete.ID_CPU, len(paquete.DISPONIBLE)))
		paquete.DISPONIBLE <- 1
		//slog.Warn(fmt.Sprintf("Después de escribir en paquete.DISPONIBLE con ID: %s, len: %d", paquete.ID_CPU, len(paquete.DISPONIBLE)))

		go loopCPU(&paquete)
		slog.Debug("no conozco cpu, agrego valor al channel, inicio goroutine")
	}

	slog.Debug("(antes) MutexInterrupcionesCpu")
	mutexInterrupcionesCPU.Lock()
	if cpupendienteInterrupcion[paquete.ID_CPU] {
		select {
		case <-InterrumpirCPU:
			slog.Debug("Señal de InterrumpirCPU consumida en handshake")
		default:
			slog.Debug("No había señal pendiente en InterrumpirCPU en handshake")
		}
		delete(cpupendienteInterrupcion, paquete.ID_CPU)
	}
	mutexInterrupcionesCPU.Unlock()
	//slog.Warn("(despues) MutexInterrupcionesCpu")
	//slog.Warn("(antes) mutexCPUporProceso")
	mutexCPUporProceso.Lock()
	slog.Debug(fmt.Sprintf("## CPU por proceso antes del delete: %v", CPUporProceso))
	delete(CPUporProceso, paquete.ID_CPU)
	slog.Debug(fmt.Sprintf("## CPU por proceso despues del delete: %v", CPUporProceso))
	mutexCPUporProceso.Unlock()

	//slog.Warn(fmt.Sprintf("Antes de escribir en cpu.DISPONIBLE con ID: %s, len: %d", cpu.ID_CPU, len(cpu.DISPONIBLE)))
	if loConozco {
		cpu.DISPONIBLE <- 1
	}
	//slog.Warn(fmt.Sprintf("Después de escribir en cpu.DISPONIBLE con ID %s, len: %d", cpu.ID_CPU, len(cpu.DISPONIBLE)))

	//slog.Warn("(despues) mutexCPUporProceso")

	//CpusDisponibles <- 1 // agrega una CPU disponible al canal
	slog.Debug("CPUS DISPONIBLES (atender handshake + 1)")

	//slog.Warn("(antes) mutexColaReady")
	mutexColaReady.Lock()
	if len(*ColaReady) != 0 {
		ProcesosEnReady <- 1
	}
	mutexColaReady.Unlock()
	//slog.Warn("(despues) mutexColaReady")

	slog.Debug(fmt.Sprintf("Conexiones CPU: %v", ConexionesCPU))

	//log.Printf("%+v\n", paquete) imprime el handshake del cpu
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func loopCPU(cpu *globales.HandshakeCPU) {
	//slog.Info(fmt.Sprintf("CPU CONECTADA %v", cpu.CONECTADA))
	for cpu.CONECTADA {
		//slog.Info(fmt.Sprintf("CPU CONECTADA %v", cpu.CONECTADA))

		_, err := buscarCPUConId(cpu.ID_CPU)
		if err != nil {
			slog.Error(fmt.Sprintf("CPU %s desconectada, no se planifica", cpu.ID_CPU))
			cpu.CONECTADA = false // desconectar
			return
		}
		switch algoritmoColaReady {
		case "FIFO":
			<-ProcesosEnReady
			planificarSinEstimador(cpu)
		case "SJF":
			slog.Debug("antes de planificar con estimadores")
			<-ProcesosEnReady
			planificarConEstimador(cpu)
		case "SRT":
			<-ProcesosEnReady
			planificarConEstimador(cpu)
		//case "SRT":
		default:
			//slog.Debug("Ya hay una señal pendiente en InterrumpirCPU, no se envía otra")
		}

	}
}

func finalizadorDeProcesos() {
	for {
		<-ProcesosAFinalizar
		slog.Debug("Adentro de finalizador de procesos")
		mutexProcesosEsperandoAFinalizar.Lock()
		if len(*ProcesosEsperandoAFinalizar) == 0 {
			mutexProcesosEsperandoAFinalizar.Unlock()
			continue
		}
		pcb := (*ProcesosEsperandoAFinalizar)[0]
		*ProcesosEsperandoAFinalizar = (*ProcesosEsperandoAFinalizar)[1:]

		cola := BuscarColaPorPID(pcb.PID)
		if cola == nil {
			slog.Error(fmt.Sprintf("No se encontró el PCB del PID %d en la cola que se suponia que estaba", pcb.PID))
			*ProcesosEsperandoAFinalizar = append(*ProcesosEsperandoAFinalizar, pcb)
			//VerificadorEstadoProcesos()
			mutexProcesosEsperandoAFinalizar.Unlock()
			ProcesosAFinalizar <- 1
			continue

		}
		pcb, err := buscarPCBYSacarDeCola(pcb.PID, cola)
		if err != nil {
			slog.Error(fmt.Sprintf("No se encontró el PCB del PID %d en la cola %s", pcb.PID, obtenerEstadoDeCola(cola)))
			*ProcesosEsperandoAFinalizar = append(*ProcesosEsperandoAFinalizar, pcb)
			//VerificadorEstadoProcesos()
			mutexProcesosEsperandoAFinalizar.Unlock()
			ProcesosAFinalizar <- 1
			continue
		}

		mutex, _ := mutexCorrespondiente(cola)
		mutex.Lock()
		*cola = append(*cola, pcb)
		mutex.Unlock()

		if !FinalizarProceso(pcb.PID, cola) {
			slog.Error(fmt.Sprintf("No se pudo finalizar el proceso con PID %d", pcb.PID))
			*ProcesosEsperandoAFinalizar = append(*ProcesosEsperandoAFinalizar, pcb)
			//VerificadorEstadoProcesos()
			mutex, _ := mutexCorrespondiente(cola)
			mutex.Lock()
			*cola = append(*cola, pcb)
			mutex.Unlock()
			mutexProcesosEsperandoAFinalizar.Unlock()
			ProcesosAFinalizar <- 1
			continue
		}
		mutexProcesosEsperandoAFinalizar.Unlock()
	}
}

func planificarSinEstimador(cpu *globales.HandshakeCPU) {

	<-cpu.DISPONIBLE

	mutexCPUporProceso.Lock()
	CPUporProceso[cpu.ID_CPU] = -1
	mutexCPUporProceso.Unlock()

	pcbReady, err := LeerPCBDesdeCola(ColaReady)
	if err != nil {
		mutexCPUporProceso.Lock()
		slog.Debug("despues del lock cpu por proceso 3")
		delete(CPUporProceso, cpu.ID_CPU)
		mutexCPUporProceso.Unlock()

		cpu.DISPONIBLE <- 1
		return
	}

	mutexCPUporProceso.Lock()

	if CPUporProceso[cpu.ID_CPU] != -1 {
		mutexCPUporProceso.Unlock()
		slog.Error(fmt.Sprintf("CPU %s inesperadamente ocupada", cpu.ID_CPU))

		cpu.DISPONIBLE <- 1

		ReinsertarEnFrenteCola(ColaReady, pcbReady)
		ProcesosEnReady <- 1
		return

	}
	CPUporProceso[cpu.ID_CPU] = pcbReady.PID
	mutexCPUporProceso.Unlock()

	slog.Debug(fmt.Sprintf("Asignando PID %d a CPU %s", pcbReady.PID, cpu.ID_CPU))

	go EnviarProcesoACPU(pcbReady, cpu)

}

func planificarConEstimador(cpu *globales.HandshakeCPU) {
	//<-EsperandoInterrupcion
	//slog.Warn(fmt.Sprintf("Valor del channel de disponibilidad de cpu antes del wait cpu %s: %v", cpu.ID_CPU, len(cpu.DISPONIBLE)))

	<-cpu.DISPONIBLE
	//<-Planificando
	//slog.Warn(fmt.Sprintf("Valor del channel de disponibilidad de cpu despues del wait cpu %s: %v", cpu.ID_CPU, len(cpu.DISPONIBLE)))
	slog.Debug("Planificar Con desalojo 2")

	//planificadorCortoPlazo.Lock()
	slog.Debug("paso mutex corto plazo (planificarConDesalojo 2)")
	//defer planificadorCortoPlazo.Unlock()

	mutexCPUporProceso.Lock()
	CPUporProceso[cpu.ID_CPU] = -1
	mutexCPUporProceso.Unlock()
	slog.Debug("despues del lock cpu por proceso 1")

	pcbReady, errReady := obtenerMenorEstimadoDeReady()
	slog.Debug("buscar en cola ready")
	if errReady != nil {
		slog.Debug("despues del lock cpu por proceso 2")
		mutexCPUporProceso.Lock()
		delete(CPUporProceso, cpu.ID_CPU)
		mutexCPUporProceso.Unlock()
		//Planificando <- 1
		cpu.DISPONIBLE <- 1
		//CpusDisponibles <- 1 // liberar CPU que se había tomado
		//EsperandoInterrupcion <- 1
		return
		//break
	}

	_, err := buscarPCBYSacarDeCola(pcbReady.PID, ColaReady)
	if err != nil {
		mutexCPUporProceso.Lock()
		slog.Debug("despues del lock cpu por proceso 3")
		delete(CPUporProceso, cpu.ID_CPU)
		mutexCPUporProceso.Unlock()
		//ProcesosEnReady <- 1
		slog.Debug("Channel procesos en ready En Planificador con desalojo (+1)")
		//Planificando <- 1
		cpu.DISPONIBLE <- 1
		//CpusDisponibles <- 1 // liberar CPU que se había tomado
		//EsperandoInterrupcion <- 1
		return

	}

	mutexCPUporProceso.Lock()
	if CPUporProceso[cpu.ID_CPU] != -1 {
		mutexCPUporProceso.Unlock()
		slog.Error(fmt.Sprintf("CPU %s inesperadamente ocupada", cpu.ID_CPU))

		ReinsertarEnFrenteCola(ColaReady, pcbReady)
		cpu.DISPONIBLE <- 1
		//Planificando <- 1
		ProcesosEnReady <- 1
		return

	}
	CPUporProceso[cpu.ID_CPU] = pcbReady.PID
	mutexCPUporProceso.Unlock()

	slog.Debug(fmt.Sprintf("Asignando PID %d a CPU %s", pcbReady.PID, cpu.ID_CPU))

	go EnviarProcesoACPU(pcbReady, cpu)
	//Planificando <- 1 // libera el canal de planificando
}

func EnviarProcesoACPU(pcb *PCB, cpu *globales.HandshakeCPU) {
	peticionCPU := globales.ProcesoAEjecutar{
		PID: pcb.PID,
		PC:  pcb.PC,
	}

	ip := cpu.IP_CPU
	puerto := cpu.PORT_CPU
	slog.Debug(fmt.Sprintf("El ID del CPU es %s, Puerto %d", cpu.ID_CPU, puerto))
	url := fmt.Sprintf("/cpu/%s/ejecutarProceso", cpu.ID_CPU)

	slog.Debug("Intentando enviar pcb a cpu ...")

	slog.Info(fmt.Sprintf("## (%d) Pasa del estado READY al estado RUNNING", pcb.PID))
	AgregarPCBaCola(pcb, ColaRunning)

	/*
		mutexCPUporProceso.Lock()
		if len(ConexionesCPU) <= len(CPUporProceso) {
			if algoritmoColaReady == "SRT" {
				select {
				case InterrumpirCPU <- 1:
					slog.Debug("Señal enviada a InterrumpirCPU")
				default:
					slog.Debug("Ya hay una señal pendiente en InterrumpirCPU, no se envía otra")
				}
			}
		}
		mutexCPUporProceso.Unlock()*/

	resp, _ := globales.GenerarYEnviarPaquete(&peticionCPU, ip, puerto, url)
	if resp.StatusCode != 200 {
		slog.Error(fmt.Sprintf("Error al enviar el proceso a la CPU: %s", resp.Status))
		//CpusDisponibles <- 1 // vuelvo a liberar el canal de cpus disponibles
		mutexCPUporProceso.Lock()
		delete(CPUporProceso, cpu.ID_CPU)
		mutexCPUporProceso.Unlock()
		cpu.DISPONIBLE <- 1
		buscarPCBYSacarDeCola(pcb.PID, ColaRunning)
		ReinsertarEnFrenteCola(ColaReady, pcb)
		ProcesosEnReady <- 1
		return
	}
}

func conozcoCPU(id_cpu string) (bool, *globales.HandshakeCPU) {
	for _, cpu := range ConexionesCPU {
		if cpu.ID_CPU == id_cpu {
			return true, &cpu
		}
	}
	return false, &globales.HandshakeCPU{}
}

func DesconectarCPU(w http.ResponseWriter, r *http.Request) {
	paquete := globales.HandshakeCPU{}
	paquete = globales.DecodificarPaquete(w, r, &paquete)

	slog.Debug(fmt.Sprintf("Desconectando CPU con ID: %s", paquete.ID_CPU))

	mutexConexionesCPU.Lock()
	for i, cpu := range ConexionesCPU {
		if cpu.ID_CPU == paquete.ID_CPU {
			//cpu.CONECTADA = false // marca la CPU como desconectada
			//ConexionesCPU[i].CONECTADA = false
			ConexionesCPU = append(ConexionesCPU[:i], ConexionesCPU[i+1:]...)
			mutexCPUporProceso.Lock()
			delete(CPUporProceso, paquete.ID_CPU)
			mutexCPUporProceso.Unlock()
			break
		}
	}
	mutexConexionesCPU.Unlock()
	// cerramos el canal de disponibilidad de la CPU
	//<-CpusDisponibles
	slog.Debug("CPUS DISPONIBLES (desconectar cpu - 1)")

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func IniciarProceso(w http.ResponseWriter, r *http.Request) {
	paquete := globales.SolicitudProceso{}
	paquete = globales.DecodificarPaquete(w, r, &paquete)

	slog.Info(fmt.Sprintf("## (%d) - Solicitó syscall - INIT_PROC", paquete.PID)) // log obligatorio

	go CrearProceso(paquete.ARCHIVO_PSEUDOCODIGO, paquete.TAMAÑO_PROCESO)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func TerminarProceso(w http.ResponseWriter, r *http.Request) {
	pid := globales.PID{}
	pid = globales.DecodificarPaquete(w, r, &pid)

	slog.Info(fmt.Sprintf("## (%d) - Solicitó syscall - EXIT", pid.NUMERO_PID)) // log obligatorio

	slog.Debug(fmt.Sprintf("Finalizando proceso (terminar proceso) con PID: %d", pid))
	//planificadorCortoPlazo.Lock()
	FinalizarProceso(pid.NUMERO_PID, ColaRunning)

	/* idcpu, err := buscarCPUConPid(pid.NUMERO_PID)
	if err != nil {
		slog.Error(fmt.Sprintf("No se encontró la CPU con PID %d", pid.NUMERO_PID))
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("CPU no encontrada"))
		return
	}
	mutexCPUporProceso.Lock()
	slog.Debug(fmt.Sprintf("## CPU por proceso antes del delete: %v", CPUporProceso))
	delete(CPUporProceso, idcpu)
	slog.Debug(fmt.Sprintf("## CPU por proceso despues del delete: %v", CPUporProceso))
	mutexCPUporProceso.Unlock() */

	//CpusDisponibles <- 1 // libera una CPU disponible

	//planificadorCortoPlazo.Unlock()
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))

}

func FinalizarProceso(pid int, cola *[]*PCB) bool {
	slog.Debug(fmt.Sprintf("Cola READY (finalizar proceso): %v \n", &ColaReady))
	slog.Debug(fmt.Sprintf("Cola RUNNING (finalizar proceso): %v \n", &ColaRunning))
	slog.Debug(fmt.Sprintf("Cola EXIT (finalizar proceso): %v \n", &ColaExit))

	slog.Debug(fmt.Sprintf("Finalizando proceso (finalizar proceso) con PID: %d", pid))
	pcb, err := buscarPCBYSacarDeCola(pid, cola)
	if err != nil {
		slog.Error(fmt.Sprintf("No se encontró el PCB del PID %d en la cola", pid))
		return false
	} else {
		pid_a_eliminar := globales.PID{
			NUMERO_PID: pid,
		}

		// peticion a memoria para liberar el espacio
		slog.Debug(fmt.Sprintf("Eliminando proceso con PID: %d de memoria", pid))
		globales.GenerarYEnviarPaquete(&pid_a_eliminar, ClientConfig.IP_MEMORY, ClientConfig.PORT_MEMORY, "/kernel/finalizar_proceso")
		slog.Debug(fmt.Sprintf("Se elimino proceso con PID: %d de memoria", pid))
		AgregarPCBaCola(pcb, ColaExit)

		slog.Info(fmt.Sprintf("## (%d) - Finaliza el proceso \n", pid)) // log obligatorio

		actualizarEsperandoFinalizacion(ColaSuspendedReady)
		actualizarEsperandoFinalizacion(ColaNew)
		ImprimirMetricasProceso(*pcb)
		return true
	}
}

// Una vez finalizado un proceso, le avisamos a los que no tenían espacio en memoria que pueden intentar entrar
func actualizarEsperandoFinalizacion(cola *[]*PCB) {
	for _, pcb := range *cola {
		if pcb.EsperandoFinalizacionDeOtroProceso {
			pcb.EsperandoFinalizacionDeOtroProceso = false
			slog.Debug(fmt.Sprintf("## (%d) - Proceso %d puede intentar entrar a memoria", pcb.PID, pcb.PID))
		}
	}
}

func DumpearMemoria(w http.ResponseWriter, r *http.Request) {
	paquete := globales.SolicitudDump{}
	paquete = globales.DecodificarPaquete(w, r, &paquete)

	slog.Info(fmt.Sprintf("## (%d) - Solicitó syscall - DUMP MEMORY", paquete.PID)) // log obligatorio

	pidABloquear := paquete.PID
	pc := paquete.PC

	/*_, err := buscarCPUConPid(pidABloquear)
	if err != nil {
		slog.Error(fmt.Sprintf("No se encontró la CPU con PID %d", pidABloquear))
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("CPU no encontrada"))
		return
	}*/

	pcbABloquear, err := buscarPCBYSacarDeCola(pidABloquear, ColaRunning)
	if err != nil {
		slog.Error(fmt.Sprintf("No se encontró el PCB del PID %d en la cola", pidABloquear))
	}
	/* 	mutexCPUporProceso.Lock()
	   	delete(CPUporProceso, id_cpu)
	   	mutexCPUporProceso.Unlock() */

	//CpusDisponibles <- 1

	pcbABloquear.PC = pc
	recalcularEstimados(pcbABloquear) // recalculo el estimado del pcb
	//AgregarPCBaCola(pcbABloquear, ColaBlocked)
	PasarAEstadoBlocked(pcbABloquear)
	slog.Info(fmt.Sprintf("## (%d) Pasa del estado RUNNING al estado BLOCKED", pidABloquear))

	peticion := globales.PID{
		NUMERO_PID: pidABloquear,
	}

	var peticionEnviada, _ = globales.GenerarYEnviarPaquete(&peticion, ClientConfig.IP_MEMORY, ClientConfig.PORT_MEMORY, "/kernel/dump_de_proceso")

	for _, p := range *ProcesosSiendoSwapeados {
		if p.PID == pidABloquear {
			slog.Debug(fmt.Sprintf("## (%d) - Eliminado de la lista de procesos en swap", p.PID))
			<-p.EstaEnSwap
			p.EstaEnSwap <- 1
			break
		}
	}

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

			pudoDesalojar, cpu := intentarDesalojo(pcbADesbloquear)
			if pudoDesalojar {
				ReinsertarEnFrenteCola(ColaReady, pcbADesbloquear)
				actualizarMetricasEstado(pcbADesbloquear, "READY")
				planificarConEstimador(cpu)
				//ProcesosEnReady <- 1
			} else {

				AgregarPCBaCola(pcbADesbloquear, ColaReady)
				//ProcesoLlegaAReady <- 1
				// ProcesosEnReady <- 1 // notifica que hay un proceso en ready
				mutexOrdenandoColaReady.Lock()
				ordenarColaReady()
				mutexOrdenandoColaReady.Unlock()
			}
			ProcesosEnReady <- 1

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
		RafagaAnterior:                     0,
		EsperandoFinalizacionDeOtroProceso: false,
		EstaEnSwap:                         make(chan int, 1),
	}
	pcb.EstaEnSwap <- 1

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

	mutexColaReady.Unlock()

}

func recalcularEstimados(pcb *PCB) {
	pcb.EstimadoAnterior = pcb.EstimadoActual
	pcb.EstimadoActual = (pcb.RafagaAnterior * alfa) + (pcb.EstimadoAnterior)*(1-alfa)
	pcb.RafagaAnterior = 0
}

// envia peticion a memoria para q mueva un proceso a swap
func EnviarProcesoASwap(pcb *PCB) bool {
	peticion := globales.PID{
		NUMERO_PID: pcb.PID,
	}

	ip := ClientConfig.IP_MEMORY
	puerto := ClientConfig.PORT_MEMORY

	slog.Debug(fmt.Sprintf("EL PROCESO %d ESTA ENVIANDOSE A SWAP", pcb.PID))

	*ProcesosSiendoSwapeados = append(*ProcesosSiendoSwapeados, pcb)
	globales.GenerarYEnviarPaquete(&peticion, ip, puerto, "/kernel/suspender_proceso")
	for i, p := range *ProcesosSiendoSwapeados {
		if p.PID == pcb.PID {
			*ProcesosSiendoSwapeados = append((*ProcesosSiendoSwapeados)[:i], (*ProcesosSiendoSwapeados)[i+1:]...)
			slog.Debug(fmt.Sprintf("EL PROCESO %d VOLVIO DE SWAP", pcb.PID))
			break
		}
	}
	return true
}

// inicia todos los planificadores
func IniciarPlanificadores() {
	PlanificadorActivo = true

	go PlanificadorLargoPlazo()
	//go PlanificadorMedianoPlazo()
	//go PlanificadorCortoPlazo()
	//go VerificadorEstadoProcesos()
	go finalizadorDeProcesos()
	slog.Debug("Planificadores iniciados: largo, corto y mediano plazo")
}

func VerificadorEstadoProcesos() {
	go func() {
		for PlanificadorActivo {
			time.Sleep(3 * time.Second)
			mutexColaBlocked.Lock()
			mutexColaSuspendedBlocked.Lock()
			mutexColaReady.Lock()
			mutexColaSuspendedReady.Lock()

			blocked := len(*ColaBlocked)
			suspendedBlocked := len(*ColaSuspendedBlocked)
			ready := len(*ColaReady)
			suspendedReady := len(*ColaSuspendedReady)

			valorChannelReady := len(ProcesosEnReady)
			valorChannelSuspendedReady := len(ProcesosEnSuspendedReady)
			//valorChannelCompartido := len(PlanificadorDeLargoPlazo)
			valorChannelNew := len(ProcesosEnNew)

			mutexColaSuspendedReady.Unlock()
			mutexColaReady.Unlock()
			mutexColaSuspendedBlocked.Unlock()
			mutexColaBlocked.Unlock()

			slog.Info("[VERIFICADOR] Estado de colas:")
			slog.Info(fmt.Sprintf("  BLOCKED: %d", blocked))
			slog.Info(fmt.Sprintf("  SUSPENDED_BLOCKED: %d", suspendedBlocked))
			slog.Info(fmt.Sprintf("  READY: %d", ready))
			slog.Info(fmt.Sprintf("  TAMANIO CANAL READY: %d", valorChannelReady))
			slog.Info(fmt.Sprintf("  SUSPENDED_READY: %d", suspendedReady))
			//slog.Warn(fmt.Sprintf("  TAMANIO CANAL SUSPREADYNEW: %d", valorChannelCompartido))
			slog.Info(fmt.Sprintf("  TAMANIO CANAL SUSPENDED READY: %d", valorChannelSuspendedReady))
			slog.Info(fmt.Sprintf("  TAMANIO CANAL NEW: %d", valorChannelNew))
			// slog.Info(fmt.Sprintf("  TAMANIO CANAL PROCESO_LLEGA_READY: %d", len(ProcesoLlegaAReady)))
			slog.Info(fmt.Sprintf(" INTERRUMPIENDO CPU: %d", len(InterrumpirCPU)))

			if suspendedBlocked > 0 && ready == 0 {
				//slog.Debug("[VERIFICADOR] Hay procesos en SUSPENDED_BLOCKED pero ninguno en READY")
			}

			if ready > 0 && len(ProcesosEnReady) == 0 {
				//slog.Debug("[VERIFICADOR] Hay procesos en READY sin ser planificados")
				ProcesosEnReady <- 1
			}

		}
	}()
}

func PlanificadorLargoPlazo() {
    for PlanificadorActivo {
        select {
        case <-ProcesosEnSuspendedReady:
			slog.Info("1")
            atenderColaSuspendidosReady()

        case <-ProcesosEnNew:
			slog.Info("2")
            // Verificación adicional para evitar interferencia
            mutexColaSuspendedReady.Lock()
            haySuspReady := len(*ColaSuspendedReady) > 0
            mutexColaSuspendedReady.Unlock()
			slog.Info("3")

            if haySuspReady {
                // Si hay procesos en SUSPENDED_READY, no atender NEW
                // Simplemente esperar la próxima señal (NO reinsertar en el canal)
                continue
            }
			slog.Info("4")
 			mutexColaNew.Lock()
			slog.Info("5")
			noHayEnNew := len(*ColaNew) == 0
			
            if noHayEnNew {
				slog.Info("6")
				mutexColaNew.Unlock()
                continue
            }
			
			slog.Info("7")
            pcb := (*ColaNew)[0]
			mutexColaNew.Unlock()
            if pcb.EsperandoFinalizacionDeOtroProceso {
				slog.Info("8")
                //ProcesosEnNew <- 1 // reinsertar en el canal de procesos en new
                slog.Debug(fmt.Sprintf("## (%d) Proceso en NEW esperando finalización de otro proceso", pcb.PID))
                continue
            }

            pcb, err := LeerPCBDesdeCola(ColaNew)
            if err != nil {
                continue
            }

            if CrearProcesoEnMemoria(pcb) {

                pudoDesalojar, cpu := intentarDesalojo(pcb)
                if pudoDesalojar {
                    ReinsertarEnFrenteCola(ColaReady, pcb)
                    actualizarMetricasEstado(pcb, "READY")
                    planificarConEstimador(cpu)
                    //ProcesosEnReady <- 1
                } else {

                    AgregarPCBaCola(pcb, ColaReady)
                    //ProcesoLlegaAReady <- 1
                    // ProcesosEnReady <- 1 // notifica que hay un proceso en ready
                    mutexOrdenandoColaReady.Lock()
                    ordenarColaReady()
                    mutexOrdenandoColaReady.Unlock()

                }
                ProcesosEnReady <- 1
                slog.Info(fmt.Sprintf("## (%d) Pasa del estado NEW al estado READY", pcb.PID))
            } else {
                AgregarPCBaCola(pcb, ColaNew)
                ordenarColaNew()
            }
        }
    }
}
/*
func PlanificadorLargoPlazo() {
    for PlanificadorActivo {
        select {
        case <-ProcesosEnSuspendedReady:
            atenderColaSuspendidosReady()

        case <-ProcesosEnNew:
            // leer PCB de ColaNew sin usar len(), y solo si hay procesos realmente
            mutexColaNew.Lock()
            if len(*ColaNew) == 0 {
                mutexColaNew.Unlock()
                continue
            }

            pcb := (*ColaNew)[0]
            if pcb.EsperandoFinalizacionDeOtroProceso {
                mutexColaNew.Unlock()
                // reinsertar al canal, ya que aún no puede planificarse
                go func() { ProcesosEnNew <- 1 }()
                slog.Debug(fmt.Sprintf("## (%d) Proceso en NEW esperando finalización de otro proceso", pcb.PID))
                continue
            }

            pcb, err := LeerPCBDesdeCola(ColaNew)
            mutexColaNew.Unlock()

            if err != nil {
                continue
            }

            if CrearProcesoEnMemoria(pcb) {
                pudoDesalojar, cpu := intentarDesalojo(pcb)
                if pudoDesalojar {
                    ReinsertarEnFrenteCola(ColaReady, pcb)
                    actualizarMetricasEstado(pcb, "READY")
                    planificarConEstimador(cpu)
                } else {
                    AgregarPCBaCola(pcb, ColaReady)
                    mutexOrdenandoColaReady.Lock()
                    ordenarColaReady()
                    mutexOrdenandoColaReady.Unlock()
                }
                ProcesosEnReady <- 1
                slog.Info(fmt.Sprintf("## (%d) Pasa del estado NEW al estado READY", pcb.PID))
            } else {
                AgregarPCBaCola(pcb, ColaNew)
                ordenarColaNew()
            }
        }
    }
}

*/

func intentarDesalojo(pcbReady *PCB) (bool, *globales.HandshakeCPU) {
	if algoritmoColaReady == "SRT" && len(ConexionesCPU) <= len(CPUporProceso) {
		var tiempoRestante float32
		pcbMasLento, errRunning := obtenerMayorEstimadoDeRunning()
		if errRunning == nil {

			tiempoRestante = pcbMasLento.EstimadoActual - float32(time.Since(pcbMasLento.TiempoInicioEstado).Milliseconds())

			slog.Info(fmt.Sprintf("PID de Running: %d, TiempoRestante: %f, PID de READY: %d, EstimadoActual: %f", pcbMasLento.PID, tiempoRestante, pcbReady.PID, pcbReady.EstimadoActual))
		}
		if errRunning == nil &&
			pcbReady.EstimadoActual < tiempoRestante { // Si tus estimados están en segundos

			slog.Debug("SRTTTT 1")

			cpuEjecutando, err2 := buscarCPUConPid(pcbMasLento.PID)
			if err2 == nil && cpuEjecutando != "" {
				slog.Debug("Encontre cpu a interrumpir")
				mutexInterrupcionesCPU.Lock()
				yaInterrumpida := cpupendienteInterrupcion[cpuEjecutando]
				slog.Debug("SRTTTT 2")
				if !yaInterrumpida {
					cpupendienteInterrupcion[cpuEjecutando] = true
				}
				mutexInterrupcionesCPU.Unlock()

				if yaInterrumpida {
					slog.Debug(fmt.Sprintf("Ya se envió interrupción a la CPU %s, no se repite.", cpuEjecutando))
					//planificadorCortoPlazo.Unlock()
					//ProcesosEnReady <- 1
					slog.Debug("valor channel procesos en ready (planificarConDesalojo, interrupcion + 1)")
					return false, nil
				}
				slog.Debug("SRTTTT 3")

				InterrumpirProceso(pcbMasLento, cpuEjecutando)
				slog.Info(fmt.Sprintf("## (%d) - Desalojado por algoritmo SJF/SRT", pcbMasLento.PID))
				cpu, _ := buscarCPUConId(cpuEjecutando)
				return true, cpu
				//planificadorCortoPlazo.Unlock()
			}
		}
		return false, nil
	}
	return false, nil
}

// si hay procesos suspendidos ready intenta pasarlos a ready
func atenderColaSuspendidosReady() {
	slog.Info("Atendiendo cola de procesos en SUSPENDED_READY")
	if len(*ColaSuspendedReady) == 0 {
		return
	}
	//<-ProcesosEnSuspendedReady
	slog.Debug(fmt.Sprintf("Valor channel procesos en suspened ready (atenderColaSuspendidosReady - 1): %d", len(ProcesosEnSuspendedReady)))

	mutexColaSuspendedReady.Lock()
	pcb := (*ColaSuspendedReady)[0]

	if pcb.EsperandoFinalizacionDeOtroProceso {
		mutexColaSuspendedReady.Unlock()
		ProcesosEnSuspendedReady <- 1
		slog.Debug(fmt.Sprintf("## (%d) Proceso en SUSPENDED_READY esperando finalización de otro proceso", pcb.PID))
		return
	}
	(*ColaSuspendedReady) = (*ColaSuspendedReady)[1:]
	mutexColaSuspendedReady.Unlock()

	/*
		pcb, err := LeerPCBDesdeCola(ColaSuspendedReady)
		if err != nil {
			//ProcesosEnSuspendedReady <- 1 // si no hay procesos en suspended ready, salgo
			return
		}*/

	// siempre true por ahora
	<-pcb.EstaEnSwap

	//go func(pcb *PCB) {
		inicializado := desuspenderProceso(pcb)
		if inicializado {

			pudoDesalojar, cpu := intentarDesalojo(pcb)
			if pudoDesalojar {
				ReinsertarEnFrenteCola(ColaReady, pcb)
				actualizarMetricasEstado(pcb, "READY")
				planificarConEstimador(cpu)
				//ProcesosEnReady <- 1
			} else {
				AgregarPCBaCola(pcb, ColaReady)
				//ProcesoLlegaAReady <- 1
				// ProcesosEnReady <- 1 // notifica que hay un proceso en ready

				mutexOrdenandoColaReady.Lock()
				ordenarColaReady()
				mutexOrdenandoColaReady.Unlock()
			}
			ProcesosEnReady <- 1

			pcb.EstaEnSwap <- 1
			slog.Info(fmt.Sprintf("## (%d) Pasa del estado SUSPENDED_READY al estado READY", pcb.PID))
		} else {
			AgregarPCBaCola(pcb, ColaSuspendedReady)
			pcb.EstaEnSwap <- 1
			ordenarColaSuspendedReady()
		}
	//}(pcb)

}

func desuspenderProceso(pcb *PCB) bool {
	peticion := globales.PID{
		NUMERO_PID: pcb.PID,
	}

	resp, _ := globales.GenerarYEnviarPaquete(&peticion, ClientConfig.IP_MEMORY, ClientConfig.PORT_MEMORY, "/kernel/dessuspender_proceso")

	return resp.StatusCode == 200
}

func BuscarCPULibre() (globales.HandshakeCPU, error) {
	// Lock both mutexes in a consistent order, copy the data, unlock, then process
	mutexConexionesCPU.Lock()
	cpus := make([]globales.HandshakeCPU, len(ConexionesCPU))
	copy(cpus, ConexionesCPU)
	mutexConexionesCPU.Unlock()

	mutexCPUporProceso.Lock()

	slog.Debug(fmt.Sprintf("CPUS CONECTADAS: %v", cpus))

	// Buscar la primera CPU no ocupada
	for _, cpu := range cpus {
		//slog.Info(fmt.Sprintf("Cpu revisada: %v", cpu))
		if _, ocupada := CPUporProceso[cpu.ID_CPU]; !ocupada {
			//	slog.Info(fmt.Sprintf("CPU libre encontrada: %s", cpu.ID_CPU))
			mutexCPUporProceso.Unlock()
			return cpu, nil
		}
	}
	mutexCPUporProceso.Unlock()

	//slog.Warn("No hay CPUs disponibles")
	return globales.HandshakeCPU{}, fmt.Errorf("no hay CPUs disponibles")
}

func InterrumpirProceso(pcb *PCB, id_cpu string) {

	slog.Debug(fmt.Sprintf("Enviando interrupción a CPU %s para desalojar PID %d", id_cpu, pcb.PID))

	cpu, err := buscarCPUConId(id_cpu)
	if err != nil {
		slog.Error(fmt.Sprintf("No se encontró la CPU con ID %s: %v", id_cpu, err))

		mutexInterrupcionesCPU.Lock()
		delete(cpupendienteInterrupcion, id_cpu)
		mutexInterrupcionesCPU.Unlock()
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
		mutexInterrupcionesCPU.Lock()
		delete(cpupendienteInterrupcion, id_cpu)
		mutexInterrupcionesCPU.Unlock()
		InterrumpirCPU <- 1
		return
	}

	slog.Debug(fmt.Sprintf("Interrupción enviada correctamente a CPU %s para PID %d", cpu.ID_CPU, pcb.PID))

}

// Devuelve el PCB con menor estimado de la cola READY
func obtenerMenorEstimadoDeReady() (*PCB, error) {
	mutexOrdenandoColaReady.Lock()
	defer mutexOrdenandoColaReady.Unlock()
	mutexColaReady.Lock()
	if len(*ColaReady) == 0 {
		mutexColaReady.Unlock()
		return nil, fmt.Errorf("cola READY vacía")
	}
	mutexColaReady.Unlock()
	ordenarColaReady()

	slog.Debug("Cola READY ordenada (obtenerMenorEstimado)")
	for _, p := range *ColaReady {
		slog.Debug(fmt.Sprintf("PID: %d, Estimado Actual: %f", p.PID, p.EstimadoActual))
	}

	return (*ColaReady)[0], nil
}

// Devuelve el PCB con mayor estimado de la cola RUNNING
func obtenerMayorEstimadoDeRunning() (*PCB, error) {
	mutexColaRunning.Lock()
	defer mutexColaRunning.Unlock()
	if len(*ColaRunning) == 0 {
		return nil, fmt.Errorf("cola RUNNING vacía")
	}
	max := (*ColaRunning)[0]
	for _, p := range *ColaRunning {
		if p.EstimadoActual > max.EstimadoActual {
			max = p
		}
	}
	return max, nil
}

// planificador de mediano plazo
func PasarAEstadoBlocked(pcb *PCB) {
	pcb.TiempoInicioEstado = time.Now()
	AgregarPCBaCola(pcb, ColaBlocked)
	slog.Debug(fmt.Sprintf("COLA BLOCKED: %v", ColaBlocked))

	// Lanza una goroutine para manejar la suspensión automática
	go iniciarTimerSuspension(pcb)
}

func iniciarTimerSuspension(pcb *PCB) {
	tiempo := time.Duration(ClientConfig.SUSPENSION_TIME) * time.Millisecond
	slog.Debug(fmt.Sprintf("INICIA TIMER PARA PROCESO %d", pcb.PID))
	time.Sleep(tiempo)
	slog.Debug(fmt.Sprintf("FINALIZA TIMER PARA PROCESO %d", pcb.PID))
	// Verifico si sigue en BLOCKED

	pcbASuspender, err := buscarPCBYSacarDeCola(pcb.PID, ColaBlocked)

	slog.Debug(fmt.Sprintf("PROCESO A SUSPENDER %v", pcbASuspender))
	if err == nil {
		<-pcbASuspender.EstaEnSwap
		swapExitoso := EnviarProcesoASwap(pcb)

		if swapExitoso {
			AgregarPCBaCola(pcbASuspender, ColaSuspendedBlocked)
			pcbASuspender.EstaEnSwap <- 1
			slog.Info(fmt.Sprintf("## (%d) Pasa de BLOCKED a SUSPENDED_BLOCKED", pcb.PID))
		} else {
			// Si falla el swap, lo devuelvo a BLOCKED
			//AgregarPCBaCola(pcbASuspender, ColaBlocked)
			pcbASuspender.EstaEnSwap <- 1
			go iniciarTimerSuspension(pcbASuspender)
			slog.Error(fmt.Sprintf("## (%d) Falló swap, se mantiene en BLOCKED", pcb.PID))
		}
		return
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
	slog.Debug(fmt.Sprintf("\nEstimado Anterior: %f, Estimado Actual: %f",
		pcb.EstimadoAnterior, pcb.EstimadoActual))
}

func mandarProcesoAIO(instancia *InstanciaIO, dispositivoIO *DispositivoIO) {
	slog.Debug(fmt.Sprintf("## Dispositivo IO %s inicio goroutine", dispositivoIO.Nombre))
	slog.Debug(fmt.Sprintf("Puntero de la instancia en mandarProcesoAIO %p", instancia))
	cola := &dispositivoIO.Cola
	//ipIO := instancia.IP
	//puertoIO := instancia.Puerto

	for instancia.EstaConectada {
		//time.Sleep(1 * time.Second) // espera 1 segundo antes de verificar la cola nuevamente
		slog.Debug(fmt.Sprintf("La IO esta Conectada, estaDisponible: %v", instancia.EstaDisponible))
		_, ok := <-instancia.EstaDisponible
		if !ok {
			slog.Debug(fmt.Sprintf("Canal de disponibilidad cerrado para %s:%d, terminando goroutine", instancia.IP, instancia.Puerto))
			break
		}
		//<-instancia.EstaDisponible
		slog.Debug("La IO esta Disponible")
		//slog.Debug(fmt.Sprintf("antes del channel"))
		<-dispositivoIO.procesosEsperandoIO
		//slog.Debug(fmt.Sprintf("despues del channel"))
		slog.Debug(fmt.Sprintf("## Dispositivo IO %s revisando cola %v", dispositivoIO.Nombre, (*cola)))
		dispositivoIO.MutexCola.Lock()
		if len(*cola) > 0 {
			proceso := (*cola)[0]
			(*cola) = (*cola)[1:]
			dispositivoIO.MutexCola.Unlock()
			/*
				if !instancia.EstaConectada {
					slog.Debug(fmt.Sprintf("Instancia %s:%d desconectada, proceso %d no ejecuta IO", instancia.IP, instancia.Puerto, proceso.PCB.PID))
					// Devolver el proceso a READY/SUSPENDED_READY
					pcb, err := buscarPCBYSacarDeCola(proceso.PCB.PID, ColaBlocked)
					if err == nil {
						mutexOrdenandoColaReady.Lock()
						AgregarPCBaCola(pcb, ColaReady)
						ProcesosEnReady <- 1
						ordenarColaReady()
						mutexOrdenandoColaReady.Unlock()
					} else {
						pcb, err := buscarPCBYSacarDeCola(proceso.PCB.PID, ColaSuspendedBlocked)
						if err == nil {
							AgregarPCBaCola(pcb, ColaSuspendedReady)
							ordenarColaSuspendedReady()
						}
					}
					break
				}*/

			// Usar puntero a instancia para modificar el mismo valor compartido
			peticionEnviada := EnviarPeticionIO(proceso.PCB, instancia.IP, instancia.Puerto, proceso.Tiempo)

			slog.Debug(fmt.Sprintf("## valor de peticion enviada: %t", peticionEnviada))

			if !peticionEnviada {
				slog.Debug(fmt.Sprintf("## Error al enviar la peticion de IO al dispositivo %s", dispositivoIO.Nombre))
				if(len(dispositivoIO.Instancias) > 0) {
					dispositivoIO.MutexCola.Lock()
					(*cola) = append([]*ProcesoEsperandoIO{proceso}, (*cola)...) 
					dispositivoIO.MutexCola.Unlock()
				} else {
					colaDelProceso := BuscarColaPorPID(proceso.PCB.PID)
					FinalizarProceso(proceso.PCB.PID, colaDelProceso)
				}
				//FinalizarProceso(proceso.PCB.PID, ColaBlocked)
			}
			// Solo marcar como disponible si la petición fue exitosa y la instancia sigue conectada
			if instancia.EstaConectada {
				select {
				case instancia.EstaDisponible <- 1:
					// Canal no cerrado, instancia marcada como disponible
				default:
					// Canal cerrado o lleno, la instancia ya no está disponible
					break
				}
			}

		} else {
			dispositivoIO.MutexCola.Unlock()
		}
	}
}

func IO(w http.ResponseWriter, r *http.Request) {
	paquete := globales.SolicitudIO{}
	paquete = globales.DecodificarPaquete(w, r, &paquete)

	go SolicitarIO(paquete.PID, paquete.PC, paquete.NOMBRE, paquete.TIEMPO)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
	slog.Debug("RESPUESTA ESCRITA EN IO")
}

func SolicitarIO(PID int, PC int, nombreIO string, tiempo int) {
	var dispositivoEncontrado bool = false
	var ioDevice *DispositivoIO

	slog.Info(fmt.Sprintf("## (%d) - Solicitó syscall - IO", PID)) // log obligatorio

	slog.Debug(fmt.Sprintf("Recibido solicitud de syscall IO: %s", nombreIO))

	//Buscar el dispositivo por nombre
	mutexDispositivosIO.Lock()
	for _, dispositivo := range DispositivosIO {
		if dispositivo.Nombre == nombreIO {
			ioDevice = dispositivo
			dispositivoEncontrado = true
			//mutexDispositivosIO.Unlock()
			break
		}
	}
	mutexDispositivosIO.Unlock()
	// Verficar si el DispositivoIO existe
	if !dispositivoEncontrado || len(ioDevice.Instancias) == 0 {
		slog.Error(fmt.Sprintf("No encuentro el dispositivo IO %s", nombreIO))
		FinalizarProceso(PID, ColaRunning)
		return
	}

	pcbABloquear, err := buscarPCBYSacarDeCola(PID, ColaRunning)
	if err != nil {
		slog.Error(fmt.Sprintf("No se encontró el PCB del PID %d a bloquear en la cola", PID))
		return
	}

	recalcularEstimados(pcbABloquear) // recalculo estimados antes de bloquear
	slog.Debug("Antes de bloquear el pcb")
	PasarAEstadoBlocked(pcbABloquear)
	slog.Debug("Despues de bloquear el pcb")

	slog.Info(fmt.Sprintf("## (%d) - Bloqueado por IO: %s", pcbABloquear.PID, nombreIO)) // log obligatorio
	slog.Info(fmt.Sprintf("## (%d) Pasa del estado RUNNING al estado BLOCKED", pcbABloquear.PID))

	(*pcbABloquear).PC = PC

	procesoEsperandoIO := ProcesoEsperandoIO{
		PCB:    pcbABloquear,
		Tiempo: tiempo,
	}

	ioDevice.MutexCola.Lock()
	ioDevice.Cola = append(ioDevice.Cola, &procesoEsperandoIO) // Agregar el proceso a la cola del dispositivo IO
	ioDevice.MutexCola.Unlock()
	slog.Debug(fmt.Sprintf("## (%d) - Agregado a la cola del dispositivo IO %s", pcbABloquear.PID, nombreIO))
	slog.Debug(fmt.Sprintf("## Cola del dispositivo IO %s: %+v", nombreIO, ioDevice.Cola))

	slog.Debug("despues del MutexCola")
	ioDevice.procesosEsperandoIO <- 1
	slog.Debug("despues del channel procesos esperando io")

}

// guarda los IO q se conectan
func AtenderHandshakeIO(w http.ResponseWriter, r *http.Request) {
	paquete := HandshakeIO{}
	paquete = globales.DecodificarPaquete(w, r, &paquete)

	slog.Debug(fmt.Sprintf("Recibido handshake del dispositivo IO: %s", paquete.Nombre))

	instancia := &InstanciaIO{
		IP:             paquete.IP,
		Puerto:         paquete.Puerto,
		EstaDisponible: make(chan int, 1),
		EstaConectada:  true,
	}
	instancia.EstaDisponible <- 1 // la instancia esta disponible al inicio

	slog.Debug(fmt.Sprintf("Puntero de la instancia cuando se crea IO %p", instancia))

	mutexDispositivosIO.Lock()
	for _, dispositivo := range DispositivosIO {
		if dispositivo.Nombre == paquete.Nombre {
			dispositivo.Instancias = append(dispositivo.Instancias, instancia)
			mutexDispositivosIO.Unlock()
			slog.Debug(fmt.Sprintf("Dispositivo IO registrado: %+v\n", paquete))

			go mandarProcesoAIO(instancia, dispositivo) // mandar goroutine para atender el dispositivo IO

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
			return
		}
	}
	mutexDispositivosIO.Unlock()

	dispositivoIO := &DispositivoIO{
		Nombre:              paquete.Nombre,
		Cola:                make([]*ProcesoEsperandoIO, 0),
		MutexCola:           new(sync.Mutex),
		TengoInstancias:     true,
		Instancias:          []*InstanciaIO{instancia},
		procesosEsperandoIO: make(chan int, 60),
	}
	///dispositivoIO.Instancias = append(dispositivoIO.Instancias, &instancia)

	mutexDispositivosIO.Lock()
	DispositivosIO = append(DispositivosIO, dispositivoIO)
	mutexDispositivosIO.Unlock()

	slog.Debug(fmt.Sprintf("Dispositivo IO registrado: %+v\n", paquete))

	go mandarProcesoAIO(instancia, dispositivoIO)

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

	return resp.StatusCode == 200 // si la respuesta es 200, retorno true, sino false
}

func AtenderFinIOPeticion(w http.ResponseWriter, r *http.Request) {
	paquete := RespuestaIO{}
	paquete = globales.DecodificarPaquete(w, r, &paquete)

	// busco el pcb en bloqueados

	pidFinIO := paquete.PID
	nombreIO := paquete.Nombre_Dispositivo
	ip := paquete.IP
	puerto := paquete.Puerto

	slog.Debug(fmt.Sprintf("IO VOLVIO PORQUE: %s", paquete.Motivo)) // log obligatorio

	if paquete.Motivo == "Desconexion" {
		DesconectarInstancia(paquete)
		slog.Info(fmt.Sprintf("## (%d) - Desconectado del dispositivo IO %s", pidFinIO, nombreIO)) 

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
		return
	}
	slog.Info(fmt.Sprintf("## (%d) finalizó IO y pasa a READY", paquete.PID)) // log obligatorio
	// Motivo = "Finalizo IO"

	for _, p := range *ProcesosSiendoSwapeados {
		if p.PID == pidFinIO {
			slog.Debug(fmt.Sprintf("## (%d) - Eliminado de la lista de procesos en swap", p.PID))
			<-p.EstaEnSwap
			p.EstaEnSwap <- 1
			slog.Debug("despues del semaforo swap")
			break
		}
	}
	slog.Debug("Sacando pcb de cola blocked")
	pcb, err := buscarPCBYSacarDeCola(pidFinIO, ColaBlocked)

	if err == nil {
		go liberarInstanciaIO(ip, puerto, nombreIO)

		pudoDesalojar, cpu := intentarDesalojo(pcb)
		if pudoDesalojar {
			ReinsertarEnFrenteCola(ColaReady, pcb)
			actualizarMetricasEstado(pcb, "READY")
			planificarConEstimador(cpu)
			//ProcesosEnReady <- 1
		} else {
			AgregarPCBaCola(pcb, ColaReady)
			//ProcesoLlegaAReady <- 1
			// ProcesosEnReady <- 1 // notifica que hay un proceso en ready

			mutexOrdenandoColaReady.Lock()
			ordenarColaReady()

			mutexOrdenandoColaReady.Unlock()
		}
		ProcesosEnReady <- 1

		slog.Info(fmt.Sprintf("## (%d) Pasa del estado BLOCKED al estado READY", pcb.PID))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))

		return
	} else {
		pcb, err := buscarPCBYSacarDeCola(pidFinIO, ColaSuspendedBlocked)
		if err == nil {
			go liberarInstanciaIO(ip, puerto, nombreIO)
			AgregarPCBaCola(pcb, ColaSuspendedReady)
			ordenarColaSuspendedReady()
			slog.Info(fmt.Sprintf("## (%d) Pasa del estado SUSPENDED_BLOCKED al estado SUSPENDED_READY", pcb.PID))

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
			return
		} else {
			go liberarInstanciaIO(ip, puerto, nombreIO)

			slog.Error(fmt.Sprintf("No se encontró el PCB del PID %d en las colas blocked/suspended_Blocked", pidFinIO))
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("pcb no encontrado"))
			return
		}
	}

}

func liberarInstanciaIO(ip string, puerto int, nombreDispositivo string) {
	for i, dispositivo := range DispositivosIO {
		if dispositivo.Nombre == nombreDispositivo {
			for j, instancia := range dispositivo.Instancias {
				if instancia.IP == ip && instancia.Puerto == puerto {
					slog.Debug(fmt.Sprintf("Puntero de la instancia cuando finaliza IO %p", instancia))
					DispositivosIO[i].Instancias[j].EstaDisponible <- 1 // la instancia vuelve a estar disponible
					slog.Debug(fmt.Sprintf("Instancia %s:%d marcada como disponible", instancia.IP, instancia.Puerto))
					break
				}
			}
		}
	}
}

func DesconectarInstancia(instanciaADesconectar RespuestaIO) {
	ip := instanciaADesconectar.IP
	puerto := instanciaADesconectar.Puerto
	nombreDispositivo := instanciaADesconectar.Nombre_Dispositivo

	mutexDispositivosIO.Lock()
	for indiceDispositivo, dispositivo := range DispositivosIO {
		slog.Debug("Entre al for 1")
		if dispositivo.Nombre == nombreDispositivo {
			slog.Debug("Entre al if 1")

			for i, instancia := range dispositivo.Instancias {
				slog.Debug("Entre al for 2")

				if instancia.IP == ip && instancia.Puerto == puerto {

					slog.Debug("Entre al if 2")

					instancia.EstaConectada = false

					// Cerrar el canal de manera segura
					select {
					case <-instancia.EstaDisponible:
						// Drenar el canal si tiene valores
					default:
						// Canal vacío o ya cerrado
					}
					slog.Debug("pase el SELECT")
					mutexProcesosEsperandoAFinalizar.Lock()
					slog.Debug("Despues del lock mutex procesos esperando a finalizar (DesconectarInstancia)")
					if instanciaADesconectar.PID > -1 {
						//colaDelPid := BuscarColaPorPID(instanciaADesconectar.PID)

						slog.Debug("Sacando pcb de cola blocked")
						pcb, err := buscarPCBYSacarDeCola(instanciaADesconectar.PID, ColaBlocked)
						slog.Debug("despues de buscar pcb en cola blocked")

						if err == nil {
							*ProcesosEsperandoAFinalizar = append(*ProcesosEsperandoAFinalizar, pcb)
							slog.Debug("Antes de reinsertar en frente cola blocked")
							ReinsertarEnFrenteCola(ColaBlocked, pcb)
							slog.Debug("Despues de reinsertar en frente cola blocked")

						} else {
							slog.Debug("Sacando pcb de cola suspended_blocked")

							pcb, err := buscarPCBYSacarDeCola(instanciaADesconectar.PID, ColaSuspendedBlocked)
							slog.Debug("despues de buscar pcb en cola suspended_blocked")
							if err == nil {
								*ProcesosEsperandoAFinalizar = append(*ProcesosEsperandoAFinalizar, pcb)
								slog.Debug("Antes de reinsertar en frente cola suspended_blocked")
								ReinsertarEnFrenteCola(ColaSuspendedBlocked, pcb)
								slog.Debug("Despues de reinsertar en frente cola suspended_blocked")

							} else {

								slog.Error(fmt.Sprintf("No se encontró el PCB del PID %d en las colas blocked/suspended_Blocked", instanciaADesconectar.PID))

							}
						}
						//*ProcesosEsperandoAFinalizar = append(*ProcesosEsperandoAFinalizar, instanciaADesconectar.PID)
						//FinalizarProceso(instanciaADesconectar.PID, colaDelPid)
					}
					mutexProcesosEsperandoAFinalizar.Unlock()
					slog.Debug("Despues del mutex procesos esperando a finalizar")
					ProcesosAFinalizar <- 1
					close(instancia.EstaDisponible)
					slog.Debug(fmt.Sprintf("Instancia %s:%d desconectada", instancia.IP, instancia.Puerto))

					dispositivo.Instancias = append(dispositivo.Instancias[:i], dispositivo.Instancias[i+1:]...)
					slog.Debug(fmt.Sprintf("Desconectando instancia de dispositivo IO: %s", dispositivo.Nombre))

					break
				}
			}
			slog.Debug("No hay instancias")
			if len(dispositivo.Instancias) == 0 {
				slog.Debug("Entre al if 3")
				DispositivosIO = append(DispositivosIO[:indiceDispositivo], DispositivosIO[indiceDispositivo+1:]...)
				slog.Debug(fmt.Sprintf("Dispositivo IO %s eliminado del sistema", dispositivo.Nombre))
				// eliminar todos los procesos que estaban esperando IO en este dispositivo
				for _, proceso := range dispositivo.Cola {
					slog.Debug("entre al for 3")
					mutexProcesosEsperandoAFinalizar.Lock()
					<-proceso.PCB.EstaEnSwap
					slog.Debug(fmt.Sprintf("## (%d) - Eliminado de la lista de procesos bloqueados por IO", proceso.PCB.PID))
					//colaDelPid := BuscarColaPorPID(proceso.PCB.PID)
					//pcb, err := buscarPCBYSacarDeCola(instanciaADesconectar.PID, ColaBlocked)
					//FinalizarProceso(proceso.PCB.PID, colaDelPid)
					*ProcesosEsperandoAFinalizar = append(*ProcesosEsperandoAFinalizar, proceso.PCB)
					proceso.PCB.EstaEnSwap <- 1
					mutexProcesosEsperandoAFinalizar.Unlock()
					ProcesosAFinalizar <- 1

				}
			}
			break
		}
	}
	mutexDispositivosIO.Unlock()

}
