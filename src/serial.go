package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/tarm/serial"
)

const (
	format      = "2006-01-02T15:04:05"
	serial_port = "/dev/ttyACM0"
	baud_rate   = 9600
	sample_rate = 50
)

type SerialState struct {
	logger     zerolog.Logger
	logfile    os.File
	serialPort *serial.Port
	accel_data []Accel3D
	rot_data   []Rot3D
}

type Accel3D struct {
	X float64
	Y float64
	Z float64
}

type Rot3D struct {
	X float64
	Y float64
	Z float64
}

func newState() (*SerialState, error) {
	// Configure logging
	filename := fmt.Sprintf("interface-log-%s.json", time.Now().Format(format))
	logfile, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0777)
	if err != nil {
		fmt.Println("Unable to open log file. Exiting.")
		return nil, err
	}
	zerolog.TimeFieldFormat = time.RFC3339Nano
	logger := zerolog.New(logfile).With().Timestamp().Logger()
	logger.Info().Msg("Interface starting")

	// Configure serial port
	c := &serial.Config{Name: serial_port, Baud: baud_rate}
	stream, err := setup_stream(c, sample_rate)
	logger.Info().Msgf("Opened serial port: %s", serial_port)
	logger.Info().Msgf("Baud rate: %d", baud_rate)

	if err != nil {
		logger.Err(err)
		fmt.Println("Unable to find sensor device. Exiting")
		return nil, err
	}

	accel_data := make([]Accel3D, 0)
	rot_data := make([]Rot3D, 0)

	return &SerialState{
		logger:     logger,
		logfile:    *logfile,
		serialPort: stream,
		accel_data: accel_data,
		rot_data:   rot_data,
	}, nil
}

func (serialState *SerialState) SerialService() {
	defer serialState.logfile.Close()

	for {
		buffer := make([]byte, 128)
		current_line := make([]byte, 0)
		fmt.Printf("Received: ")
		for {
			n, err := serialState.serialPort.Read(buffer)
			if err != nil {
				serialState.logger.Err(err)
			}
			fmt.Printf("%s", string(buffer[:n]))
			for i := range n {
				current_line = append(current_line, buffer[i])
			}
			if buffer[n-1] == byte('\n') {
				break
			}
		}
		reader := csv.NewReader(strings.NewReader(string(current_line)))

		for {
			data, err := reader.Read()
			if err != nil {
				serialState.logger.Err(err)
			}
			if err == io.EOF {
				break
			}

			tmp_accel, err := newAccel3D(data[:3])
			if err != nil {
				serialState.logger.Err(err)
			}
			serialState.accel_data = append(serialState.accel_data, *tmp_accel)

			tmp_rot, err := newRot3D(data[3:6])
			if err != nil {
				serialState.logger.Err(err)
			}
			serialState.rot_data = append(serialState.rot_data, *tmp_rot)
		}

		if len(serialState.accel_data) != 0 && len(serialState.rot_data) != 0 {
			serialState.logger.Info().Msgf("%v,%v",
				serialState.accel_data[len(serialState.accel_data)-1],
				serialState.rot_data[len(serialState.rot_data)-1])
		}

		if len(serialState.accel_data) > 16 {
			serialState.accel_data = removeAccel(serialState.accel_data)
		}

		if len(serialState.rot_data) > 16 {
			serialState.rot_data = removeRot(serialState.rot_data)
		}
	}
}

func (a *Accel3D) Print() {
	fmt.Printf("%f,%f,%f", a.X, a.Y, a.Z)
}

func (r *Rot3D) Print() {
	fmt.Printf("%f,%f,%f", r.X, r.Y, r.Z)
}

func (a *Accel3D) String() string {
	return fmt.Sprintf("%f,%f,%f", a.X, a.Y, a.Z)
}

func (r *Rot3D) String() string {
	return fmt.Sprintf("%f,%f,%f", r.X, r.Y, r.Z)
}

func newAccel3D(data []string) (*Accel3D, error) {
	x, err := strconv.ParseFloat(data[0], 64)
	if err != nil {
		return nil, err
	}
	y, err := strconv.ParseFloat(data[1], 64)
	if err != nil {
		return nil, err
	}
	z, err := strconv.ParseFloat(data[2], 64)
	if err != nil {
		return nil, err
	}

	return &Accel3D{
		X: x,
		Y: y,
		Z: z,
	}, nil
}

func newRot3D(data []string) (*Rot3D, error) {
	x, err := strconv.ParseFloat(data[0], 64)
	if err != nil {
		return nil, err
	}
	y, err := strconv.ParseFloat(data[1], 64)
	if err != nil {
		return nil, err
	}
	z, err := strconv.ParseFloat(data[2], 64)
	if err != nil {
		return nil, err
	}

	return &Rot3D{
		X: x,
		Y: y,
		Z: z,
	}, nil
}

func setup_stream(c *serial.Config, sample_rate int) (*serial.Port, error) {
	stream, err := serial.OpenPort(c)
	if err != nil {
		return nil, err
	}

	_, err = stream.Write([]byte("\n"))
	if err != nil {
		return nil, err
	}

	time.Sleep(10 * time.Millisecond)

	_, err = stream.Write(fmt.Appendf(nil, "%d", sample_rate))
	if err != nil {
		return nil, err
	}

	buffer := make([]byte, 128)

	for {
		fmt.Print("Received: ")
		should_exit := false
		for {
			n, err := stream.Read(buffer)
			if err != nil {
				return nil, err
			}

			fmt.Printf("%s", string(buffer[:n]))
			if buffer[n-1] == 0x02 {
				should_exit = true
				break
			}
		}
		if should_exit {
			break
		}
	}

	return stream, nil
}

func removeAccel(slice []Accel3D) []Accel3D {
	_, sliced := slice[0], slice[1:]
	return sliced
}

func removeRot(slice []Rot3D) []Rot3D {
	_, sliced := slice[0], slice[1:]
	return sliced
}

func main() {
	serialState, err := newState()
	if err != nil {
		fmt.Printf("Failed to initialize state. Error: %s\n", err)
	}

	go serialState.SerialService()
	fmt.Println("Service dispatched")
	for {
	}
}
