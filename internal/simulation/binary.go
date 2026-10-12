package simulation

import (
	"bytes"
	"encoding/binary"
	"math"
	"sync/atomic"
)

const (
	FrameTypeInit uint8 = 0x01
	FrameTypeTick uint8 = 0x02
)

// EncodeInitFrame sérialise les données statiques d'environnement (obstacles, sas, dimensions)
// émises une seule fois lors de l'établissement de la connexion WebSocket.
func (e *Engine) EncodeInitFrame(buf *bytes.Buffer) {
	buf.Reset()
	buf.WriteByte(FrameTypeInit)

	var hdr [6]byte
	binary.LittleEndian.PutUint16(hdr[0:2], uint16(WorldSize))
	binary.LittleEndian.PutUint16(hdr[2:4], uint16(GridCols))
	binary.LittleEndian.PutUint16(hdr[4:6], uint16(GridRows))
	buf.Write(hdr[:])

	// Sas (Exits)
	buf.WriteByte(byte(len(e.exits)))
	var exitBuf [16]byte
	for _, ex := range e.exits {
		binary.LittleEndian.PutUint32(exitBuf[0:4], math.Float32bits(float32(ex.X)))
		binary.LittleEndian.PutUint32(exitBuf[4:8], math.Float32bits(float32(ex.Y)))
		binary.LittleEndian.PutUint32(exitBuf[8:12], math.Float32bits(float32(ex.R)))
		binary.LittleEndian.PutUint32(exitBuf[12:16], math.Float32bits(float32(ex.ID)))
		buf.Write(exitBuf[:])
	}

	// Obstacles statiques
	var obsHdr [2]byte
	binary.LittleEndian.PutUint16(obsHdr[0:2], uint16(len(e.obstacles)))
	buf.Write(obsHdr[:])

	var obsBuf [16]byte
	for _, obs := range e.obstacles {
		binary.LittleEndian.PutUint32(obsBuf[0:4], math.Float32bits(float32(obs.X)))
		binary.LittleEndian.PutUint32(obsBuf[4:8], math.Float32bits(float32(obs.Y)))
		binary.LittleEndian.PutUint32(obsBuf[8:12], math.Float32bits(float32(obs.W)))
		binary.LittleEndian.PutUint32(obsBuf[12:16], math.Float32bits(float32(obs.H)))
		buf.Write(obsBuf[:])
	}
}

// EncodeTickFrame sérialise l'état dynamique de la simulation en format binaire compact direct (5 octets / agent).
// Retourne isFinished indiquant la fin de simulation (aucun survivant sain restant).
func (e *Engine) EncodeTickFrame(buf *bytes.Buffer, currentTPS int) (isFinished bool) {
	buf.Reset()
	buf.WriteByte(FrameTypeTick)

	healthy, survCount, normCount, alertedCount, exhaustedCount := e.GetSurvivorCounts()
	isFinished = (healthy == 0)

	esc := int(atomic.LoadInt64(&e.escaped))
	inf := int(atomic.LoadInt64(&e.infected))
	if inf < len(e.horde) {
		inf = len(e.horde)
	}

	processed := esc + (inf - InitialInfected)
	ratio := float32(0.0)
	if processed > 0 {
		ratio = float32(math.Round((float64(esc)/float64(processed))*1000.0) / 10.0)
	}

	var finishByte byte
	if isFinished {
		finishByte = 1
	}

	// Header stats fixes (35 octets de données statistiques et métriques)
	var statsHdr [34]byte
	binary.LittleEndian.PutUint64(statsHdr[0:8], uint64(MasterSeed))
	binary.LittleEndian.PutUint16(statsHdr[8:10], uint16(currentTPS))
	binary.LittleEndian.PutUint16(statsHdr[10:12], uint16(inf))
	binary.LittleEndian.PutUint16(statsHdr[12:14], uint16(healthy))
	binary.LittleEndian.PutUint16(statsHdr[14:16], uint16(alertedCount))
	binary.LittleEndian.PutUint16(statsHdr[16:18], uint16(exhaustedCount))
	binary.LittleEndian.PutUint16(statsHdr[18:20], uint16(survCount))
	binary.LittleEndian.PutUint16(statsHdr[20:22], uint16(normCount))
	binary.LittleEndian.PutUint16(statsHdr[22:24], uint16(esc))
	statsHdr[24] = byte(e.GetOpenExitsCount())
	statsHdr[25] = finishByte
	binary.LittleEndian.PutUint32(statsHdr[26:30], math.Float32bits(ratio))

	// Statut des 4 sas d'évacuation (4 octets : bit 0 = fermé, bit 1 = armé)
	for idx, ex := range e.exits {
		if idx < 4 {
			var b byte
			if ex.IsClosed {
				b |= 0x01
			}
			if ex.HasSurvivalistIn {
				b |= 0x02
			}
			statsHdr[30+idx] = b
		}
	}
	buf.Write(statsHdr[:])

	// Heatmap sparse : encodage uniquement des cellules actives (contenant >= 1 zombie)
	hm := e.GenerateHeatmap()
	activeCells := 0
	for _, count := range hm {
		if count > 0 {
			activeCells++
		}
	}

	var hmHdr [2]byte
	binary.LittleEndian.PutUint16(hmHdr[0:2], uint16(activeCells))
	buf.Write(hmHdr[:])

	var cellBuf [3]byte
	for idx, count := range hm {
		if count > 0 {
			binary.LittleEndian.PutUint16(cellBuf[0:2], uint16(idx))
			c := count
			if c > 255 {
				c = 255
			}
			cellBuf[2] = byte(c)
			buf.Write(cellBuf[:])
		}
	}

	// Survivants vivants : 2 octets pour le nombre d'agents + 5 octets par agent
	var survHdr [2]byte
	binary.LittleEndian.PutUint16(survHdr[0:2], uint16(healthy))
	buf.Write(survHdr[:])

	needed := healthy * 5
	buf.Grow(needed)
	target := buf.AvailableBuffer()
	if cap(target) >= needed {
		target = target[:needed]
		offset := 0
		for i := range e.survivors {
			s := &e.survivors[i]
			if s.Alive && !s.Escaped {
				x := uint16(int16(s.X))
				y := uint16(int16(s.Y))
				target[offset] = byte(x)
				target[offset+1] = byte(x >> 8)
				target[offset+2] = byte(y)
				target[offset+3] = byte(y >> 8)

				var flags byte
				if s.IsSurvivalist {
					flags |= 0x01
				}
				if s.IsAlerted {
					flags |= 0x02
				}
				if s.IsExhausted {
					flags |= 0x04
				}
				target[offset+4] = flags
				offset += 5
			}
		}
		buf.Write(target)
	} else {
		var sBuf [5]byte
		for i := range e.survivors {
			s := &e.survivors[i]
			if s.Alive && !s.Escaped {
				binary.LittleEndian.PutUint16(sBuf[0:2], uint16(int16(s.X)))
				binary.LittleEndian.PutUint16(sBuf[2:4], uint16(int16(s.Y)))
				var flags byte
				if s.IsSurvivalist {
					flags |= 0x01
				}
				if s.IsAlerted {
					flags |= 0x02
				}
				if s.IsExhausted {
					flags |= 0x04
				}
				sBuf[4] = flags
				buf.Write(sBuf[:])
			}
		}
	}

	return isFinished
}
