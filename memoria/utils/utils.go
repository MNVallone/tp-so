package utils

import (
	"encoding/json"
	"fmt"
	"globales/servidor"
	"log"
	"log/slog"
	"net/http"
	"os"
)

type Config struct {
	PORT_MEMORY      int    `json:"port_memory"`
	MEMORY_SIZE      int    `json:"memory_size"`
	PAGE_SIZE        int    `json:"page_size"`
	ENTRIES_PER_PAGE int    `json:"entries_per_page"`
	NUMBER_OF_LEVELS int    `json:"number_of_levels"`
	MEMORY_DELAY     int    `json:"memory_delay"`
	SWAPFILE_PATH    string `json:"swapfile_path"`
	SWAP_DELAY       string `json:"swap_delay"`
	LOG_LEVEL        string `json:"log_level"`
	DUMP_PATH        string `json:"dump_path"`
}

type EspacioMemoriaPeticion struct {
	TamanioSolicitado int `json:"tamanio_solicitado"`
}

type EspacioMemoriaRespuesta struct {
	EspacioDisponible int  `json:"espacio_disponible"`
	Exito             bool `json:"exito"`
}

var ClientConfig *Config
var EspacioUsado int = 0

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
