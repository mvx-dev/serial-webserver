package serial

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/tarm/serial"
)

const (
	Format         = "2006-01-02T15:04:05"
	Default_serial = "/dev/ttyACM0"
	baud_rate      = 9600
	sample_rate    = 50
	max_retention  = int(1000 / sample_rate) // This is to ensure the delta speed is delayed by a second
)

type SerialState struct {
	logger        zerolog.Logger
	logfile       os.File
	serialPort    *serial.Port
	accel_data    []Accel3D
	rot_data      []Rot3D
	rolling_speed float64
	rolling_delta float64
	start_time    time.Time
	Channel       chan []float64
}

type Accel3D struct {
	X float64
	Y float64
	Z float64
	T int64
}

type Rot3D struct {
	X float64
	Y float64
	Z float64
}

func NewState(serial_port string) (*SerialState, error) {
	// Configure logging
	filename := fmt.Sprintf("interface-log-%s.json", time.Now().Format(Format))
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
		logger:        logger,
		logfile:       *logfile,
		serialPort:    stream,
		accel_data:    accel_data,
		rot_data:      rot_data,
		rolling_speed: 0,
		rolling_delta: 0,
		start_time:    time.Now(),
		Channel:       make(chan []float64),
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

			delta_t := time.Since(serialState.start_time)
			tmp_accel, err := newAccel3D(append(data[:3], strconv.Itoa(int(delta_t))))
			if err != nil {
				serialState.logger.Err(err)
			}
			serialState.accel_data = append(serialState.accel_data, *tmp_accel)

			tmp_rot, err := newRot3D(data[3:6])
			if err != nil {
				serialState.logger.Err(err)
			}
			serialState.rot_data = append(serialState.rot_data, *tmp_rot)

			serialState.rolling_speed += tmp_accel.GetSpeed()
			serialState.rolling_delta = serialState.rolling_speed - serialState.accel_data[0].GetSpeed()
		}

		if len(serialState.accel_data) != 0 && len(serialState.rot_data) != 0 {
			serialState.logger.Info().Msgf("%v,%v",
				serialState.accel_data[len(serialState.accel_data)-1],
				serialState.rot_data[len(serialState.rot_data)-1])
		}

		if len(serialState.accel_data) > max_retention {
			serialState.accel_data = removeAccel(serialState.accel_data)
		}

		if len(serialState.rot_data) > max_retention {
			serialState.rot_data = removeRot(serialState.rot_data)
		}

		serialState.Channel <- serialState.Format()
	}
}

func (serialState *SerialState) Format() []float64 {
	values := make([]float64, 0)
	accel := serialState.accel_data[len(serialState.accel_data)-1]
	rot := serialState.rot_data[len(serialState.rot_data)-1]

	values = append(values, accel.ToArray()...)        // Raw acceleration
	values = append(values, accel.Abs())               // Total acceleration
	values = append(values, serialState.rolling_delta) // Speed delta
	values = append(values, serialState.rolling_speed) // Absolute speed
	values = append(values, rot.ToArray()...)          // Raw rotation
	values = append(values, float64(time.Since(serialState.start_time)))

	return values
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

func (a *Accel3D) ToArray() []float64 {
	return []float64{a.X, a.Y, a.Z}
}

func (r *Rot3D) ToArray() []float64 {
	return []float64{r.X, r.Y, r.Z}
}

func (a *Accel3D) Abs() float64 {
	return math.Sqrt(math.Pow(a.X, 2) + math.Pow(a.Y, 2) + math.Pow(a.Z, 2))
}

func (a *Accel3D) GetSpeed() float64 {
	return a.Abs() * float64(a.T)
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
