package utils

// Si el nombre de una funcion/variable empieza con una letra mayuscula, es porque es exportable
// Si empieza con una letra minuscula, es porque es privada al paquete

import (
	cpuUtils "cpu/utils"
	"encoding/json"
	"fmt"
	"globales"
	"globales/servidor"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"
)

type Config struct {
	IP_MEMORY           string  `json:"ip_memory"`
	IP_KERNEL           string  `json:"ip_kernel"`
	PORT_MEMORY         int     `json:"port_memory"`
	PORT_KERNEL         int     `json:"port_kernel"`
	SCHEDULER_ALGORITHM string  `json:"scheduler_algorithm"`
	READY_ALGORITHM     string  `json:"ready_ingress_algorithm"`
	ALPHA               float64 `json:"alpha"`
	SUSPENSION_TIME     int     `json:"suspension_time"`
	LOG_LEVEL           string  `json:"log_level"`
}

var ClientConfig *Config

// ruta y tamaño del proceso inicial
var RutaInicial string
var TamanioInicial int

type Paquete struct {
	Valores string `json:"valores"`
}

func IniciarConfiguracion(filePath string) *Config {
	var config *Config
	configFile, err := os.Open(filePath)
	if err != nil {
		slog.Error(err.Error())
	}
	defer configFile.Close()

	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&config)

	return config
}

// Colas de estado de los procesos
var ColaNew []globales.PCB
var ColaReady []globales.PCB
var ColaRunning []globales.PCB
var ColaBlocked []globales.PCB
var ColaSuspendedBlocked []globales.PCB
var ColaSuspendedReady []globales.PCB
var ColaExit []globales.PCB

var PlanificadorActivo bool = false

var CPUsDisponibles []cpuUtils.Handshake

var EstimacionProcesos map[int]float64

var UltimoPID int = 0

type ProcesoSuspension struct {
	PID           int
	TiempoBloqueo time.Time
}

var ProcesosBlocked []ProcesoSuspension

type PeticionMemoria struct {
	PID     int    `json:"pid"`
	Tamanio int    `json:"tamanio"`
	Ruta    string `json:"ruta"`
}

type RespuestaMemoria struct {
	Exito   bool   `json:"exito"`
	Mensaje string `json:"mensaje"`
}

type PeticionCPU struct {
	PID int `json:"pid"`
	PC  int `json:"pc"`
}

type PeticionSwap struct {
	PID    int    `json:"pid"`
	Accion string `json:"accion"` // SWAP_OUT o SWAP_IN
}

func AgregarPCBaCola(pcb globales.PCB, cola *[]globales.PCB) {
	//cola.Lock()
	//defer cola.Unlock()
	*cola = append(*cola, pcb)
	slog.Info(fmt.Sprintf("globales.PCB agregado a la cola: %v", pcb))
}

func LeerPCBDesdeCola(cola *[]globales.PCB) (globales.PCB, error) {
	// cola.Lock()
	// defer cola.Unlock()
	if len(*cola) > 0 {
		pcb := (*cola)[0]
		*cola = (*cola)[1:]
		slog.Info(fmt.Sprintf("PCB leido desde la cola: %v", pcb))
		return pcb, nil
	} else {
		slog.Info("No hay PCBs en la cola")
		return globales.PCB{}, fmt.Errorf("no hay PCBs en la cola")
	}
}

func CambiarDeEstado(origen *[]globales.PCB, destino *[]globales.PCB) {
	// origen.Lock()
	// defer origen.Unlock()
	// destino.Lock()
	// defer destino.Unlock()
	pcb, err := LeerPCBDesdeCola(origen)
	if err == nil {
		AgregarPCBaCola(pcb, destino)
		var nombreOrigen, nombreDestino = traducirNombresColas(origen, destino)
		slog.Info(fmt.Sprintf("PCB movido de %v a %v: %v", nombreOrigen, nombreDestino, pcb))
	} else {
		slog.Info(fmt.Sprintf("No hay PCBs en la cola %v", origen))
	}
}

func traducirNombresColas(origen *[]globales.PCB, destino *[]globales.PCB) (string, string) {
	var nombreOrigen string = ""
	var nombreDestino string = ""
	switch origen {
	case &ColaNew:
		nombreOrigen = "ColaNew"
	case &ColaReady:
		nombreOrigen = "ColaReady"
	case &ColaRunning:
		nombreOrigen = "ColaRunning"
	case &ColaBlocked:
		nombreOrigen = "ColaBlocked"
	case &ColaSuspendedBlocked:
		nombreOrigen = "ColaSuspendedBlocked"
	case &ColaSuspendedReady:
		nombreOrigen = "ColaSuspendedReady"
	}
	switch destino {
	case &ColaNew:
		nombreDestino = "ColaNew"
	case &ColaReady:
		nombreDestino = "ColaReady"
	case &ColaRunning:
		nombreDestino = "ColaRunning"
	case &ColaBlocked:
		nombreDestino = "ColaBlocked"
	case &ColaSuspendedBlocked:
		nombreDestino = "ColaSuspendedBlocked"
	case &ColaSuspendedReady:
		nombreDestino = "ColaSuspendedReady"
	}
	return nombreOrigen, nombreDestino
}

func EliminarPCBaCola(pcb globales.PCB, cola *[]globales.PCB) {
	// cola.Lock()
	// defer cola.Unlock()
	for i, p := range *cola {
		if p.PID == pcb.PID {
			*cola = append((*cola)[:i], (*cola)[i+1:]...)
			slog.Info(fmt.Sprintf("PCB eliminado de la cola: %v", pcb))
			return
		}
	}
	slog.Info(fmt.Sprintf("PCB no encontrado en la cola: %v", pcb))
}

func RecibirHandshakeCpu(w http.ResponseWriter, r *http.Request) cpuUtils.Handshake {
	paquete := cpuUtils.Handshake{}
	paquete = servidor.DecodificarPaquete(w, r, &paquete)

	return paquete
}

func AtenderCPU(w http.ResponseWriter, r *http.Request) {
	var paquete servidor.PCB = servidor.RecibirPaquetesCpu(w, r)
	slog.Info("Recibido paquete CPU")
	log.Printf("%+v\n", paquete)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func AtenderHandshakeCPU(w http.ResponseWriter, r *http.Request) {
	var paquete cpuUtils.Handshake = RecibirHandshakeCpu(w, r)
	slog.Info(fmt.Sprintf("Recibido handshake CPU %d", paquete.ID_CPU))

	// guardo la cpu en la lista d disponibles
	CPUsDisponibles = append(CPUsDisponibles, paquete)
	log.Printf("CPU registrada: %+v\n", paquete)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

type HandshakeIO struct {
	Nombre string `json:"nombre"`
	IP     string `json:"ip"`
	Puerto int    `json:"puerto"`
}

// lista de ios q se conectaron
var DispositivosIO []HandshakeIO

// recibe handshake de io
func RecibirHandshakeIO(w http.ResponseWriter, r *http.Request) HandshakeIO {
	paquete := HandshakeIO{}
	paquete = servidor.DecodificarPaquete(w, r, &paquete)

	return paquete
}

// guarda los IO q se conectan
func AtenderHandshakeIO(w http.ResponseWriter, r *http.Request) {
	var paquete HandshakeIO = RecibirHandshakeIO(w, r)
	slog.Info(fmt.Sprintf("Recibido handshake del dispositivo IO: %s", paquete.Nombre))

	DispositivosIO = append(DispositivosIO, paquete)
	log.Printf("Dispositivo IO registrado: %+v\n", paquete)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

type RespuestaIO struct {
	PID    int    `json:"pid"`
	Estado string `json:"estado"`
}

type PeticionIO struct {
	PID    int `json:"pid"`
	Tiempo int `json:"tiempo"`
}

// lee la respuesta q manda io
func RecibirRespuestaIO(w http.ResponseWriter, r *http.Request) RespuestaIO {
	paquete := RespuestaIO{}
	paquete = servidor.DecodificarPaquete(w, r, &paquete)
	return paquete
}

// envia peticion al dispositivo io disponible
func EnviarPeticionIO(pcb globales.PCB, nombreDispositivo string, tiempoIO int) bool {
	// busco el disp io q necesito
	var dispositivoEncontrado bool = false
	var ioDevice HandshakeIO

	for _, dispositivo := range DispositivosIO {
		if dispositivo.Nombre == nombreDispositivo {
			ioDevice = dispositivo
			dispositivoEncontrado = true
			break
		}
	}

	if !dispositivoEncontrado {
		slog.Error(fmt.Sprintf("No encuentro el dispositivo IO %s", nombreDispositivo))
		return false
	}

	// armo el paquete con pid y tiempo
	peticion := PeticionIO{
		PID:    pcb.PID,
		Tiempo: tiempoIO,
	}

	slog.Info(fmt.Sprintf("## (%d) - Bloqueado por IO: %s", pcb.PID, nombreDispositivo))

	// pongo el proceso en bloqueados
	EliminarPCBaCola(pcb, &ColaRunning)
	AgregarPCBaCola(pcb, &ColaBlocked)
	slog.Info(fmt.Sprintf("## (%d) Pasa del estado RUNNING al estado BLOCKED", pcb.PID))

	// guardo el tiempo en q se bloqueo para q el plani de mediano plazo lo suspenda
	registroSuspension := ProcesoSuspension{
		PID:           pcb.PID,
		TiempoBloqueo: time.Now(),
	}
	ProcesosBlocked = append(ProcesosBlocked, registroSuspension)

	// mando la peticion al io
	ip := ioDevice.IP
	port := ioDevice.Puerto
	globales.GenerarYEnviarPaquete(&peticion, ip, port, "/io/peticion")

	return true
}

// procesa cuando termina una io
func AtenderFinIOPeticion(w http.ResponseWriter, r *http.Request) {
	var respuesta RespuestaIO = RecibirRespuestaIO(w, r)

	slog.Info(fmt.Sprintf("## (%d) finalizó IO y pasa a READY", respuesta.PID))

	// busco el pcb en bloqueados
	var pcbEncontrado bool = false
	var pcb globales.PCB

	for i, p := range ColaBlocked {
		if p.PID == respuesta.PID {
			pcb = p
			// lo saco d bloqueados
			ColaBlocked = append(ColaBlocked[:i], ColaBlocked[i+1:]...)
			pcbEncontrado = true
			break
		}
	}

	// busco y elimino el registro de tiempo de bloqueo
	for i, p := range ProcesosBlocked {
		if p.PID == respuesta.PID {
			ProcesosBlocked = append(ProcesosBlocked[:i], ProcesosBlocked[i+1:]...)
			break
		}
	}

	if !pcbEncontrado {
		// si no esta en bloqueados fijo esta en suspblocked
		for i, p := range ColaSuspendedBlocked {
			if p.PID == respuesta.PID {
				pcb = p
				// lo saco d la cola
				ColaSuspendedBlocked = append(ColaSuspendedBlocked[:i], ColaSuspendedBlocked[i+1:]...)
				// lo pongo en susp ready
				AgregarPCBaCola(pcb, &ColaSuspendedReady)
				slog.Info(fmt.Sprintf("## (%d) Pasa del estado SUSPENDED_BLOCKED al estado SUSPENDED_READY", pcb.PID))
				pcbEncontrado = true
				break
			}
		}

		if !pcbEncontrado {
			slog.Error(fmt.Sprintf("No encuentro el PCB %d en ninguna cola d bloqueados", respuesta.PID))
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("pcb no encontrado"))
			return
		}
	} else {
		// estaba en bloqueados normal
		AgregarPCBaCola(pcb, &ColaReady)
		slog.Info(fmt.Sprintf("## (%d) Pasa del estado BLOCKED al estado READY", pcb.PID))
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func CrearProceso(rutaPseudocodigo string, tamanio int) {
	UltimoPID++
	pid := UltimoPID

	slog.Info(fmt.Sprintf("## (%d) Se crea el proceso - Estado: NEW", pid))

	pcb := globales.PCB{
		PID: pid,
		PC:  0,
		ME: globales.METRICAS_KERNEL{
			NEW:               1,
			READY:             0,
			RUNNING:           0,
			BLOCKED:           0,
			SUSPENDED_BLOCKED: 0,
			SUSPENDED_READY:   0,
			EXIT:              0,
		},
		MT: globales.METRICAS_KERNEL{
			NEW:               0,
			READY:             0,
			RUNNING:           0,
			BLOCKED:           0,
			SUSPENDED_BLOCKED: 0,
			SUSPENDED_READY:   0,
			EXIT:              0,
		},
	}

	AgregarPCBaCola(pcb, &ColaNew)
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
		if len(ColaNew) == 0 {
			continue
		}

		pcb, err := LeerPCBDesdeCola(&ColaNew)
		if err != nil {
			continue
		}

		// siempre retorna true por ahora
		inicializado := InicializarProcesoEnMemoria(pcb) // propagar la ruta y el tamaño con el que se ejecuta cuando no tengamos que mockear la resp de memoria

		if inicializado {
			// actualizo metricas
			pcb.ME.NEW--
			pcb.ME.READY++

			// lo paso a ready
			AgregarPCBaCola(pcb, &ColaReady)
			slog.Info(fmt.Sprintf("## (%d) Pasa del estado NEW al estado READY", pcb.PID))
		} else {
			// no pudo inicializarse, vuelve a new
			AgregarPCBaCola(pcb, &ColaNew)
		}
	}
}

// si hay procesos suspendidos ready intenta pasarlos a ready
func atenderColaSuspendidosReady() {
	if len(ColaSuspendedReady) == 0 {
		return
	}

	pcb, err := LeerPCBDesdeCola(&ColaSuspendedReady)
	if err != nil {
		return
	}

	// siempre true por ahora
	inicializado := true

	if inicializado {
		// actualizo metricas
		pcb.ME.SUSPENDED_READY--
		pcb.ME.READY++

		// lo paso a ready
		AgregarPCBaCola(pcb, &ColaReady)
		slog.Info(fmt.Sprintf("## (%d) Pasa del estado SUSPENDED_READY al estado READY", pcb.PID))
	} else {
		// no se pudo, vuelve a cola
		AgregarPCBaCola(pcb, &ColaSuspendedReady)
	}
}

// recibe proceso que retorna de cpu
func AtenderRetornoCPU(w http.ResponseWriter, r *http.Request) {
	paquete := PeticionCPU{}
	paquete = servidor.DecodificarPaquete(w, r, &paquete)

	// busco el pcb en running por su pid
	var pcbEncontrado bool = false
	var pcb globales.PCB

	for i, p := range ColaRunning {
		if p.PID == paquete.PID {
			pcb = p
			// actualizo el pc con valor retornado
			pcb.PC = paquete.PC
			// lo saco de running
			ColaRunning = append(ColaRunning[:i], ColaRunning[i+1:]...)
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

	AgregarPCBaCola(pcb, &ColaExit)
	slog.Info(fmt.Sprintf("## (%d) Pasa del estado RUNNING al estado EXIT", pcb.PID))

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// planificador de corto plazo fifo
func PlanificadorCortoPlazo() {
	for PlanificadorActivo {
		if len(CPUsDisponibles) == 0 {
			continue
		}

		// si no hay procesos ready, sigo
		if len(ColaReady) == 0 {
			continue
		}

		pcb, err := LeerPCBDesdeCola(&ColaReady)
		if err != nil {
			continue
		}

		// actualizo metricas
		pcb.ME.READY--
		pcb.ME.RUNNING++

		// selecciona primera cpu disponible
		cpu := CPUsDisponibles[0]
		CPUsDisponibles = CPUsDisponibles[1:]

		// cambio a running
		AgregarPCBaCola(pcb, &ColaRunning)
		slog.Info(fmt.Sprintf("## (%d) Pasa del estado READY al estado RUNNING", pcb.PID))

		// armo paquete para cpu
		peticionCPU := PeticionCPU{
			PID: pcb.PID,
			PC:  pcb.PC,
		}

		// envio a cpu
		ip := cpu.IP_CPU
		puerto := cpu.PORT_CPU
		globales.GenerarYEnviarPaquete(&peticionCPU, ip, puerto, "/cpu/ejecutar")
	}
}

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

				// busco el pcb en la cola de bloqueados
				var pcbEncontrado bool = false
				var pcb globales.PCB

				for j, p := range ColaBlocked {
					if p.PID == pid {
						pcb = p
						// lo saco d bloqueados
						ColaBlocked = append(ColaBlocked[:j], ColaBlocked[j+1:]...)
						pcbEncontrado = true
						break
					}
				}

				if pcbEncontrado {
					// actualizo metricas
					pcb.ME.BLOCKED--
					pcb.ME.SUSPENDED_BLOCKED++

					// informo a memoria q lo mueva a swap
					swapExitoso := EnviarProcesoASwap(pcb)

					if swapExitoso {
						// lo paso a susp_blocked
						AgregarPCBaCola(pcb, &ColaSuspendedBlocked)
						slog.Info(fmt.Sprintf("## (%d) Pasa del estado BLOCKED al estado SUSPENDED_BLOCKED", pcb.PID))

						// elimino el registro de tiempo de bloqueo
						ProcesosBlocked = append(ProcesosBlocked[:i], ProcesosBlocked[i+1:]...)
						i-- // ajusto i porque eliminé un elemento del slice
					} else {
						// si falla lo vuelvo a poner en bloqueados
						AgregarPCBaCola(pcb, &ColaBlocked)
					}
				}
			}
		}
	}
}
