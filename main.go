package main

import (
	"errors"
	"fmt"
	mft "github.com/AlecRandazzo/MFT-Parser"
	vbr "github.com/AlecRandazzo/VBR-Parser"
	"io"
	"os"
)

type VolumeHandler struct {
	Handle               *os.File
	VolumeLetter         string
	Vbr                  vbr.VolumeBootRecord
	mftReader            io.Reader
	lastReadVolumeOffset int64
}

func main() {
	volume, err := GetVolumeHandler()
	if err != nil {
		fmt.Println("[1] Fail get volume handle")
		fmt.Println(err)
	}

	mftRecord, err := ParseMFTRecord(&volume)
	if err != nil {
		fmt.Println("[2] fail parse mft record")
		fmt.Println(err)
	}

	_record := mftRecord.FileNameAttributes[0]
	_data := mftRecord.DataAttribute
	_dataList := _data.NonResidentDataAttribute

	// fmt.Println(_record)
	fmt.Printf("FILE NAME: %s\n", _record.FileName)

	var count int64
	for _, value := range _dataList.DataRuns {
		count += value.Length
	}

	fmt.Printf("SIZE: %d\n", count)
	fmt.Printf("Start Offset: %d\n", volume.Vbr.MftByteOffset)

	// test
	_file, _ := os.Open("C:\\$MFT")
	_file.Seek(volume.Vbr.MftByteOffset, 0)

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
		err = fmt.Errorf("Failed...: %s", err)
		return
	}

	_, err = volume.Handle.Seek(volume.Vbr.MftByteOffset, 0)
	if err != nil {
		err = fmt.Errorf("Failed...: %s", err)
		return
	}

	buffer := make([]byte, volume.Vbr.MftRecordSize)
	_, err = volume.Handle.Read(buffer)
	if err != nil {
		err = fmt.Errorf("Failed...: %s", err)
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
		err = fmt.Errorf("Failed...: %s", err)
		return
	}

	return
}
