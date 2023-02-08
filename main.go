package main

import (
	"errors"
	"fmt"
	mft "github.com/AlecRandazzo/MFT-Parser"
	vbr "github.com/AlecRandazzo/VBR-Parser"
	"io"
	"os"
	"sync"
)

type VolumeHandler struct {
	Handle               *os.File
	VolumeLetter         string
	Vbr                  vbr.VolumeBootRecord
	mftReader            io.Reader
	lastReadVolumeOffset int64
}

type dataRunsReader struct {
	VolumeHandler                 *VolumeHandler
	DataRuns                      mft.DataRuns
	fileName                      string
	dataRunTracker                int
	dataRunBytesLeftToReadTracker int64
	totalFileSize                 int64
	totalByesRead                 int64
	initialized                   bool
}

type foundFile struct {
	dataRuns mft.DataRuns
	fullPath string
	fileSize int64
}

type fileReader struct {
	fullPath string
	reader   io.Reader
}

func main() {
	volume, _ := GetVolumeHandler()
	mftRecord, _ := ParseMFTRecord(&volume)

	waitForFileCopying := sync.WaitGroup{}
	waitForFileCopying.Add(1)

	chanFile := make(chan fileReader, 100)

	go Collect(chanFile, &waitForFileCopying)

	foundFile := foundFile{
		dataRuns: mftRecord.DataAttribute.NonResidentDataAttribute.DataRuns,
		fullPath: "$mft",
	}

	mftReader := rawFileReader(&volume, foundFile)

	// mft collect
	pipeReader, pipeWriter := io.Pipe()
	teeReader := io.TeeReader(mftReader, pipeWriter)

	fileReader := fileReader{
		fullPath: fmt.Sprintf("%s__$mft", volume.VolumeLetter),
		reader:   pipeReader,
	}

	chanFile <- fileReader
	volume.mftReader = teeReader

	fmt.Print(mftRecord)
	fmt.Print("\n------\n")
	fmt.Print(mftReader)
}

func Collect(readers chan fileReader, waitForFileCopying *sync.WaitGroup) (err error) {
	defer waitForFileCopying.Done()

	openChannel := true

	for openChannel == true {
		reader := fileReader{}
		reader, openChannel = <-readers

		if openChannel == false {
			break
		}

		path := reader.fullPath

		var writer io.Writer
		writer, _ = os.Create(path)

		io.Copy(writer, reader.reader)
	}
	err = nil
	return
}

func GetVolumeHandler() (volume VolumeHandler, err error) {
	const volumeBootRecordSize = 512
	const volumeLetter = "C"

	volume.VolumeLetter = volumeLetter
	volume.Handle, _ = os.Open(fmt.Sprintf("\\\\.\\%s:", volumeLetter))

	volumeBootRecord := make([]byte, volumeBootRecordSize)
	_, err = volume.Handle.Read(volumeBootRecord)

	volume.Vbr, err = vbr.RawVolumeBootRecord(volumeBootRecord).Parse()

	return
}

func ParseMFTRecord(volume *VolumeHandler) (mftRecord mft.MasterFileTableRecord, err error) {
	_, err = volume.Handle.Seek(0x00, 0)
	if err != nil {
		err = fmt.Errorf("Failed...: %w", err)
		return
	}

	_, err = volume.Handle.Seek(volume.Vbr.MftByteOffset, 0)
	if err != nil {
		err = fmt.Errorf("Failed...: %w", err)
		return
	}

	buffer := make([]byte, volume.Vbr.MftRecordSize)
	_, err = volume.Handle.Read(buffer)
	if err != nil {
		err = fmt.Errorf("Failed...: %w", err)
		return
	}

	result, err := mft.RawMasterFileTableRecord(buffer).IsThisAnMftRecord()

	if err != nil {
		err = fmt.Errorf("Failed...: %v", err)
	} else if result == false {
		err = errors.New("Failed...: %")
		return
	}

	mftRecord, err = mft.RawMasterFileTableRecord(buffer).Parse(volume.Vbr.BytesPerCluster)

	if err != nil {
		err = fmt.Errorf("Failed...: %w", err)
		return
	}

	return
}

// Reader.go
func rawFileReader(handler *VolumeHandler, file foundFile) (reader io.Reader) {
	reader = &dataRunsReader{
		VolumeHandler:                 handler,
		DataRuns:                      file.dataRuns,
		fileName:                      file.fullPath,
		dataRunTracker:                0,
		dataRunBytesLeftToReadTracker: 0,
		totalFileSize:                 file.fileSize,
		initialized:                   false,
	}

	return
}

// Reader.go
func (dataRunReader *dataRunsReader) Read(byteSliceToPopulate []byte) (numberOfBytesRead int, err error) {
	bufferSize := int64(len(byteSliceToPopulate))

	// Sanity checking
	if len(dataRunReader.DataRuns) == 0 {
		err = io.ErrUnexpectedEOF
		return
	}

	// Check if this reader has been initialized, if not, do so.
	if dataRunReader.initialized != true {
		if dataRunReader.totalFileSize == 0 {
			for _, dataRun := range dataRunReader.DataRuns {
				dataRunReader.totalFileSize += dataRun.Length
			}
		}
		dataRunReader.dataRunTracker = 0
		dataRunReader.dataRunBytesLeftToReadTracker = dataRunReader.DataRuns[dataRunReader.dataRunTracker].Length
		dataRunReader.VolumeHandler.lastReadVolumeOffset, _ = dataRunReader.VolumeHandler.Handle.Seek(dataRunReader.DataRuns[dataRunReader.dataRunTracker].AbsoluteOffset, 0)
		dataRunReader.VolumeHandler.lastReadVolumeOffset -= bufferSize
		dataRunReader.initialized = true

	}

	// Figure out how many bytes are left to read
	if dataRunReader.dataRunBytesLeftToReadTracker-bufferSize == 0 {
		dataRunReader.dataRunBytesLeftToReadTracker -= bufferSize
	} else if dataRunReader.dataRunBytesLeftToReadTracker-bufferSize < 0 {
		bufferSize = dataRunReader.dataRunBytesLeftToReadTracker
		dataRunReader.dataRunBytesLeftToReadTracker = 0
	} else {
		dataRunReader.dataRunBytesLeftToReadTracker -= bufferSize
	}

	// Read from the data run
	if dataRunReader.totalByesRead+bufferSize > dataRunReader.totalFileSize {
		bufferSize = dataRunReader.totalFileSize - dataRunReader.totalByesRead
	}
	buffer := make([]byte, bufferSize)
	dataRunReader.VolumeHandler.lastReadVolumeOffset += bufferSize
	numberOfBytesRead, _ = dataRunReader.VolumeHandler.Handle.Read(buffer)
	copy(byteSliceToPopulate, buffer)
	dataRunReader.totalByesRead += bufferSize
	if dataRunReader.totalFileSize == dataRunReader.totalByesRead {
		err = io.EOF
		return
	}

	// Check to see if there are any bytes left to read in the current data run
	if dataRunReader.dataRunBytesLeftToReadTracker == 0 {
		// Increment our tracker
		dataRunReader.dataRunTracker++

		// Get the size of the next datarun
		dataRunReader.dataRunBytesLeftToReadTracker = dataRunReader.DataRuns[dataRunReader.dataRunTracker].Length

		// Seek to the offset of the next datarun
		dataRunReader.VolumeHandler.lastReadVolumeOffset, _ = dataRunReader.VolumeHandler.Handle.Seek(dataRunReader.DataRuns[dataRunReader.dataRunTracker].AbsoluteOffset, 0)
		dataRunReader.VolumeHandler.lastReadVolumeOffset -= bufferSize
	}

	return
}
