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
)

type Config struct {
	IP_MEMORY           string `json:"ip_memory"`
	PORT_MEMORY         int    `json:"port_memory"`
	PORT_KERNEL         int    `json:"port_kernel"`
	SCHEDULER_ALGORITHM string `json:"sheduler_algorithm"`
	NEW_ALGORITHM       string `json:"new_algorithm"`
	ALPHA               int    `json:"alpha"`
	SUSPENSION_TIME     int    `json:"suspension_time"`
	LOG_LEVEL           string `json:"log_level"`
}

var ClientConfig *Config

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

//TODO: implementar semaforo para modificar colas de PCBs

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
	slog.Info("Recibido handshake CPU.")

	// To do: Implementar la logica del handshake.

	log.Printf("%+v\n", paquete)
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
