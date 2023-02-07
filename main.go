package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	vbr "github.com/AlecRandazzo/VBR-Parser"
	mft "github.com/AlecRandazzo/MFT-Parser"
)

type VolumeHandler struct {
	Handle 					*os.File
	VolumeLetter 			string
	Vbr 					vbr.VolumeBootRecord
	mftReader 				io.Reader
	lastReadVolumeOffset 	int64
}

func main() {
	volume, _ := GetVolumeHandler()
	mftRecord, _ := ParseMFTRecord(&volume)

	fmt.Print(mftRecord)
}


func GetVolumeHandler () (volume VolumeHandler, err error) {
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
