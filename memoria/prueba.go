package main

// import (
// 	"bytes"
// 	"encoding/gob"
// 	"fmt"
// 	"os"
// )

// type MyStruct struct {
// 	PID  int
// 	Data []int
// }

// func main() {
// 	myStructs := []MyStruct{
// 		{PID: 1, Data: []int{23, 2, 32, 32, 13, 21, 25, 1515, 125, 1, 512, 512, 5, 125, 125, 125, 12}},
// 		{PID: 5, Data: []int{1, 2, 123, 123, 123, 12, 312, 312, 312, 312, 3, 124, 124, 12, 4}}}

// 	buffer := new(bytes.Buffer) // Buffer de bytes
// 	encoder := gob.NewEncoder(buffer)
// 	for _, s := range myStructs {
// 		encoder.Encode(s)
// 	}
// 	data := buffer.Bytes()
// 	fmt.Println("Buffer bytes:", data)
// 	os.WriteFile("/home/utnso/Desktop/Operativos/tp-go/tp-2025-1c-Harkcoded/memoria/swapfile.bin", data, 0644)

// 	// Deserialización de los datos
// 	var deserializedStructs []MyStruct
// 	archivo, _ := os.ReadFile("/home/utnso/Desktop/Operativos/tp-go/tp-2025-1c-Harkcoded/memoria/swapfile.bin")

// 	buffer = bytes.NewBuffer(archivo)
// 	decoder := gob.NewDecoder(buffer)

// 	for {
// 		var s MyStruct
// 		err := decoder.Decode(&s)
// 		if err != nil {
// 			break
// 		}
// 		deserializedStructs = append(deserializedStructs, s)
// 	}

// 	// Imprimir los datos deserializados
// 	fmt.Println("Deserialized structs:")
// 	for _, s := range deserializedStructs {
// 		fmt.Printf("PID: %d, Data: %v\n", s.PID, s.Data)
// 	}
// }
