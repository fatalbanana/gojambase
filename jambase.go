package jambase

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"time"
)

var (
	jamSignature = []byte{74, 65, 77, 00}
	revisionOne  = uint16(1)
)

type JAMFixedHeaderInfo struct {
	ActiveMessages    int
	BaseMessageNumber int
	DateCreated       time.Time
	UpdateCounter     int
}

type JAMMessageHeaderSubfield struct {
	FieldID    uint16
	FieldValue string
}

type JAMMessageHeader struct {
	DateReceived  time.Time
	DateProcessed time.Time
	DateWritten   time.Time
	MessageNumber uint32
	NumSubfields  uint32
	ReplyFirst    uint32
	ReplyNext     uint32
	ReplyTo       bool
	SubFields     []JAMMessageHeaderSubfield
	TextOffset    uint32
	TextLength    uint32
}

type JAMMessage struct {
	Header JAMMessageHeader
	Text   string
}

type JAMBase struct {
	headerFilePath   string
	textDataFilePath string
}

func (j *JAMBase) ReadMessages() (<-chan JAMMessage, <-chan error) {
	messageCh := make(chan JAMMessage, 1)
	errorCh := make(chan error, 1)

	go func() {
		defer close(errorCh)
		defer close(messageCh)

		headerHandle, err := os.Open(j.headerFilePath)
		if err != nil {
			errorCh <- err
			return
		}
		defer headerHandle.Close()

		textDataHandle, err := os.Open(j.textDataFilePath)
		if err != nil {
			errorCh <- err
			return
		}
		defer headerHandle.Close()

		fhi, err := readFixedHeader(headerHandle)
		if err != nil {
			errorCh <- err
			return
		}

		for i := 0; i < fhi.ActiveMessages; i++ {
			jamMsg := JAMMessage{}
			jamMsg.Header, err = readMessageHeader(headerHandle)
			if err != nil {
				errorCh <- err
				return
			}
			_, err = textDataHandle.Seek(int64(jamMsg.Header.TextOffset), 0)
			if err != nil {
				errorCh <- err
				return
			}
			buf := make([]byte, jamMsg.Header.TextLength)
			bytesRead, err := textDataHandle.Read(buf)
			if err != nil {
				errorCh <- err
				return
			}
			if uint32(bytesRead) != jamMsg.Header.TextLength {
				errorCh <- ErrShortRead
				return
			}
			jamMsg.Text = strings.ReplaceAll(string(buf), "\r", "\n") // XXX: hmm
			messageCh <- jamMsg
		}
	}()

	return messageCh, errorCh
}

func (j *JAMBase) ReadFixedHeader() (JAMFixedHeaderInfo, error) {
	f, err := os.Open(j.headerFilePath)
	if err != nil {
		return JAMFixedHeaderInfo{}, err
	}
	defer f.Close()
	return readFixedHeader(f)
}

func readMessageHeader(fh *os.File) (JAMMessageHeader, error) {
	res := JAMMessageHeader{}
	headerBuffer := make([]byte, 76)
	bytesRead, err := fh.Read(headerBuffer)
	if err != nil {
		return res, err
	}
	if bytesRead != len(headerBuffer) {
		return res, ErrShortRead
	}
	if !bytes.Equal(headerBuffer[0:4], jamSignature) {
		return res, ErrNoJAMSignature
	}
	revision := binary.LittleEndian.Uint16(headerBuffer[4:6])
	if revision != revisionOne {
		return res, fmt.Errorf("Unknown JAM header revision: %d", revision)
	}
	// ushort ReservedWord; // Reserved for future use
	subfieldLength := binary.LittleEndian.Uint32(headerBuffer[8:12])
	// ulong  TimesRead;    // Number of times message read
	// ulong  MSGIDcrc;     // CRC-32 of MSGID line              (3)
	// ulong  REPLYcrc;     // CRC-32 of REPLY line              (3)
	// ulong  ReplyTo;      // This msg is a reply to..
	// ulong  Reply1st;     // First reply to this msg
	// ulong  Replynext;    // Next msg in reply chain
	// ulong  DateWritten;  // When msg was written
	// ulong  DateReceived; // When msg was read by recipient
	// ulong  DateProcessed;// When msg was processed by tosser/scanner
	// ulong  MessageNumber;// Message number (1-based)
	// ulong  Attribute;    // Msg attribute, see "Msg Attributes"
	// ulong  Attribute2;   // Reserved for future use
	res.TextOffset = binary.LittleEndian.Uint32(headerBuffer[60:64])
	res.TextLength = binary.LittleEndian.Uint32(headerBuffer[64:68])
	// ulong  PasswordCRC;  // CRC-32 of password to access message
	// ulong  Cost;         // Cost of message
	buf := make([]byte, subfieldLength)
	bytesRead, err = fh.Read(buf)
	if err != nil {
		return res, err
	}
	if bytesRead != len(buf) {
		return res, ErrShortRead
	}
	// XXX: process subfields
	return res, nil
}

func readFixedHeader(fh *os.File) (JAMFixedHeaderInfo, error) {
	res := JAMFixedHeaderInfo{}
	headerBuffer := make([]byte, 1024)

	bytesRead, err := fh.Read(headerBuffer)
	if err != nil {
		return res, err
	}
	if bytesRead != len(headerBuffer) {
		return res, ErrShortRead
	}
	if !bytes.Equal(headerBuffer[0:4], jamSignature) {
		return res, ErrNoJAMSignature
	}

	dateCreated := binary.LittleEndian.Uint32(headerBuffer[4:8])
	updateCounter := binary.LittleEndian.Uint32(headerBuffer[8:12])
	activeMessages := binary.LittleEndian.Uint32(headerBuffer[12:16])
	// XXX: password crc
	baseMsgNum := binary.LittleEndian.Uint32(headerBuffer[20:24])

	res.DateCreated = time.Unix(int64(dateCreated), 0).UTC()
	res.UpdateCounter = int(updateCounter)
	res.ActiveMessages = int(activeMessages)
	res.BaseMessageNumber = int(baseMsgNum)
	return res, nil
}

func relatedPathFromHeaderPath(headerPath string, extension string) (string, error) {
	headerPathLen := len(headerPath)
	if headerPathLen < 4 {
		return "", ErrNoExtension
	}
	switch hint := headerPath[headerPathLen-4 : headerPathLen]; hint {
	case ".JHR":
		return headerPath[:headerPathLen-3] + strings.ToUpper(extension), nil
	case ".jhr":
		return headerPath[:headerPathLen-3] + extension, nil
	default:
		return "", ErrNoExtension
	}
}

func NewJAMBase(headerFilePath string) (*JAMBase, error) {
	res := &JAMBase{
		headerFilePath: headerFilePath,
	}
	var err error
	res.textDataFilePath, err = relatedPathFromHeaderPath(headerFilePath, "jdt")
	return res, err
}
