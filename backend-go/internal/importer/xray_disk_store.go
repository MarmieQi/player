package importer

import (
	"bufio"
	"container/list"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	xrayDiskFileHeader  = "XRAY2"
	xrayDiskWriterLimit = 64
	xrayDiskMaxString   = 16 * 1024 * 1024
)

type xrayDiskStore struct {
	baseDirectory string
	jobDirectory  string
	location      *time.Location
	players       map[string]string
	writers       map[string]*xrayDiskWriter
	writerLRU     *list.List
}

type xrayDiskWriter struct {
	path    string
	file    *os.File
	buffer  *bufio.Writer
	element *list.Element
}

func newXrayDiskStore(baseDirectory, jobID string, location *time.Location) (*xrayDiskStore, error) {
	baseDirectory = filepath.Clean(strings.TrimSpace(baseDirectory))
	if baseDirectory == "." || baseDirectory == "" {
		return nil, errors.New("矿透分析临时目录不能为空")
	}
	if location == nil {
		location = time.Local
	}
	if err := os.MkdirAll(baseDirectory, 0o750); err != nil {
		return nil, fmt.Errorf("创建矿透分析临时目录失败: %w", err)
	}
	jobDirectory := filepath.Join(baseDirectory, "job-"+jobID)
	if !isDirectChild(baseDirectory, jobDirectory) {
		return nil, errors.New("矿透分析任务临时目录无效")
	}
	if err := os.Mkdir(jobDirectory, 0o750); err != nil {
		return nil, fmt.Errorf("创建矿透分析任务临时目录失败: %w", err)
	}
	return &xrayDiskStore{
		baseDirectory: baseDirectory,
		jobDirectory:  jobDirectory,
		location:      location,
		players:       map[string]string{},
		writers:       map[string]*xrayDiskWriter{},
		writerLRU:     list.New(),
	}, nil
}

func (s *xrayDiskStore) accept(event xrayEvent) error {
	path, ok := s.players[event.playerName]
	if !ok {
		path = filepath.Join(s.jobDirectory, fmt.Sprintf("player-%08d.bin", len(s.players)+1))
		s.players[event.playerName] = path
	}
	writer, err := s.writer(path)
	if err != nil {
		return err
	}
	if err := writeXrayDiskEvent(writer.buffer, event); err != nil {
		return fmt.Errorf("写入玩家 %q 的矿透临时数据失败: %w", event.playerName, err)
	}
	return nil
}

func (s *xrayDiskStore) results() ([]XrayPlayerRiskView, error) {
	if err := s.closeWriters(); err != nil {
		return nil, err
	}
	playerNames := make([]string, 0, len(s.players))
	for playerName := range s.players {
		playerNames = append(playerNames, playerName)
	}
	sort.Strings(playerNames)

	results := make([]XrayPlayerRiskView, 0, len(playerNames))
	for _, playerName := range playerNames {
		events, err := readXrayDiskEvents(s.players[playerName], playerName, s.location)
		if err != nil {
			return nil, fmt.Errorf("读取玩家 %q 的矿透临时数据失败: %w", playerName, err)
		}
		sort.Slice(events, func(i, j int) bool {
			return events[i].happenedAt.Before(events[j].happenedAt)
		})
		result := analyzePlayer(playerName, events)
		if includeXrayResult(result) {
			results = append(results, result)
		}
	}
	sortXrayResults(results)
	if len(results) > 200 {
		return results[:200], nil
	}
	return results, nil
}

func (s *xrayDiskStore) cleanup() error {
	closeErr := s.closeWriters()
	removeErr := removeXrayJobDirectory(s.baseDirectory, s.jobDirectory)
	return errors.Join(closeErr, removeErr)
}

func (s *xrayDiskStore) writer(path string) (*xrayDiskWriter, error) {
	if writer, ok := s.writers[path]; ok {
		s.writerLRU.MoveToFront(writer.element)
		return writer, nil
	}
	if len(s.writers) >= xrayDiskWriterLimit {
		oldest := s.writerLRU.Back()
		if oldest != nil {
			writer := oldest.Value.(*xrayDiskWriter)
			if err := s.closeWriter(writer); err != nil {
				return nil, err
			}
		}
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("打开矿透临时文件失败: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	buffer := bufio.NewWriterSize(file, 64*1024)
	if info.Size() == 0 {
		if _, err := buffer.WriteString(xrayDiskFileHeader); err != nil {
			_ = file.Close()
			return nil, err
		}
	}
	writer := &xrayDiskWriter{path: path, file: file, buffer: buffer}
	writer.element = s.writerLRU.PushFront(writer)
	s.writers[path] = writer
	return writer, nil
}

func (s *xrayDiskStore) closeWriter(writer *xrayDiskWriter) error {
	delete(s.writers, writer.path)
	if writer.element != nil {
		s.writerLRU.Remove(writer.element)
		writer.element = nil
	}
	flushErr := writer.buffer.Flush()
	closeErr := writer.file.Close()
	return errors.Join(flushErr, closeErr)
}

func (s *xrayDiskStore) closeWriters() error {
	var closeErr error
	for s.writerLRU.Len() > 0 {
		writer := s.writerLRU.Back().Value.(*xrayDiskWriter)
		closeErr = errors.Join(closeErr, s.closeWriter(writer))
	}
	return closeErr
}

func cleanupXrayTempRoot(baseDirectory string) error {
	baseDirectory = filepath.Clean(strings.TrimSpace(baseDirectory))
	if baseDirectory == "." || baseDirectory == "" {
		return nil
	}
	entries, err := os.ReadDir(baseDirectory)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var cleanupErr error
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "job-") {
			continue
		}
		cleanupErr = errors.Join(cleanupErr, removeXrayJobDirectory(baseDirectory, filepath.Join(baseDirectory, entry.Name())))
	}
	return cleanupErr
}

func removeXrayJobDirectory(baseDirectory, jobDirectory string) error {
	if !isDirectChild(baseDirectory, jobDirectory) || !strings.HasPrefix(filepath.Base(jobDirectory), "job-") {
		return errors.New("拒绝清理无效的矿透分析任务临时目录")
	}
	if err := os.RemoveAll(jobDirectory); err != nil {
		return fmt.Errorf("清理矿透分析任务临时目录失败: %w", err)
	}
	return nil
}

func isDirectChild(parent, child string) bool {
	parentAbs, err := filepath.Abs(parent)
	if err != nil {
		return false
	}
	childAbs, err := filepath.Abs(child)
	if err != nil {
		return false
	}
	return filepath.Dir(childAbs) == parentAbs && childAbs != parentAbs
}

func includeXrayResult(result XrayPlayerRiskView) bool {
	return result.RiskScore >= 20 || result.AnalysisTrackingEvidenceCount > 0 || result.AnalysisAncientDebrisBreaks >= 3
}

func sortXrayResults(results []XrayPlayerRiskView) {
	sort.Slice(results, func(i, j int) bool {
		if results[i].RiskScore != results[j].RiskScore {
			return results[i].RiskScore > results[j].RiskScore
		}
		return results[i].PlayerName < results[j].PlayerName
	})
}

func writeXrayDiskEvent(writer io.Writer, event xrayEvent) error {
	if err := writeXrayInt64(writer, event.happenedAt.Unix()); err != nil {
		return err
	}
	if err := writeXrayInt64(writer, int64(event.happenedAt.Nanosecond())); err != nil {
		return err
	}
	if err := writeXrayInt64(writer, event.row.LineNumber); err != nil {
		return err
	}
	for _, value := range []float64{event.point.x, event.point.y, event.point.z} {
		if err := writeXrayUint64(writer, math.Float64bits(value)); err != nil {
			return err
		}
	}
	var flags byte
	if event.rareOre {
		flags |= 1 << 0
	}
	if event.trackingTarget {
		flags |= 1 << 1
	}
	if event.tunnelBlock {
		flags |= 1 << 2
	}
	if event.hasPoint {
		flags |= 1 << 3
	}
	if _, err := writer.Write([]byte{flags}); err != nil {
		return err
	}
	values := [...]string{
		event.blockID,
		event.oreType,
		event.row.FileName,
		event.row.FilePath,
		event.row.Date,
		event.row.Time,
		event.row.PlayerName,
		event.row.Action,
		event.row.X,
		event.row.Y,
		event.row.Z,
		event.row.Dimension,
		event.row.X2,
		event.row.Y2,
		event.row.Z2,
		event.row.Dimension2,
		event.row.Detail1,
		event.row.Detail2,
	}
	for _, value := range values {
		if err := writeXrayString(writer, value); err != nil {
			return err
		}
	}
	return nil
}

func readXrayDiskEvents(path, playerName string, location *time.Location) ([]xrayEvent, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := bufio.NewReaderSize(file, 64*1024)
	header := make([]byte, len(xrayDiskFileHeader))
	if _, err := io.ReadFull(reader, header); err != nil {
		return nil, err
	}
	if string(header) != xrayDiskFileHeader {
		return nil, errors.New("矿透临时文件格式无效")
	}

	var events []xrayEvent
	for {
		event, err := readXrayDiskEvent(reader, playerName, location)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

func readXrayDiskEvent(reader *bufio.Reader, playerName string, location *time.Location) (xrayEvent, error) {
	happenedAtUnix, err := readXrayInt64(reader)
	if err != nil {
		return xrayEvent{}, err
	}
	happenedAtNanosecond, err := readXrayInt64(reader)
	if err != nil {
		return xrayEvent{}, err
	}
	lineNumber, err := readXrayInt64(reader)
	if err != nil {
		return xrayEvent{}, err
	}
	pointValues := [3]float64{}
	for index := range pointValues {
		value, err := readXrayUint64(reader)
		if err != nil {
			return xrayEvent{}, err
		}
		pointValues[index] = math.Float64frombits(value)
	}
	flags, err := reader.ReadByte()
	if err != nil {
		return xrayEvent{}, err
	}
	values := make([]string, 18)
	for index := range values {
		value, err := readXrayString(reader)
		if err != nil {
			return xrayEvent{}, err
		}
		values[index] = value
	}
	row := LogQueryRow{
		FileName:   values[2],
		FilePath:   values[3],
		LineNumber: lineNumber,
		Date:       values[4],
		Time:       values[5],
		PlayerName: values[6],
		Action:     values[7],
		X:          values[8],
		Y:          values[9],
		Z:          values[10],
		Dimension:  values[11],
		X2:         values[12],
		Y2:         values[13],
		Z2:         values[14],
		Dimension2: values[15],
		Detail1:    values[16],
		Detail2:    values[17],
	}
	return xrayEvent{
		row:            row,
		happenedAt:     time.Unix(happenedAtUnix, happenedAtNanosecond).In(location),
		playerName:     playerName,
		blockID:        values[0],
		oreType:        values[1],
		rareOre:        flags&(1<<0) != 0,
		trackingTarget: flags&(1<<1) != 0,
		tunnelBlock:    flags&(1<<2) != 0,
		point:          point3{x: pointValues[0], y: pointValues[1], z: pointValues[2]},
		hasPoint:       flags&(1<<3) != 0,
	}, nil
}

func writeXrayString(writer io.Writer, value string) error {
	var length [binary.MaxVarintLen64]byte
	size := binary.PutUvarint(length[:], uint64(len(value)))
	if _, err := writer.Write(length[:size]); err != nil {
		return err
	}
	_, err := io.WriteString(writer, value)
	return err
}

func readXrayString(reader *bufio.Reader) (string, error) {
	length, err := binary.ReadUvarint(reader)
	if err != nil {
		return "", err
	}
	if length > xrayDiskMaxString {
		return "", errors.New("矿透临时文件字符串长度超出限制")
	}
	value := make([]byte, int(length))
	if _, err := io.ReadFull(reader, value); err != nil {
		return "", err
	}
	return string(value), nil
}

func writeXrayInt64(writer io.Writer, value int64) error {
	return writeXrayUint64(writer, uint64(value))
}

func writeXrayUint64(writer io.Writer, value uint64) error {
	var buffer [8]byte
	binary.LittleEndian.PutUint64(buffer[:], value)
	_, err := writer.Write(buffer[:])
	return err
}

func readXrayInt64(reader io.Reader) (int64, error) {
	value, err := readXrayUint64(reader)
	return int64(value), err
}

func readXrayUint64(reader io.Reader) (uint64, error) {
	var buffer [8]byte
	if _, err := io.ReadFull(reader, buffer[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(buffer[:]), nil
}
