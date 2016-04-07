package producer

import "time"

type RecordAccumulatorConfig struct {
	batchSize int
	linger    time.Duration
}

type RecordAccumulator struct {
	input chan *ProducerRecord

	config        *RecordAccumulatorConfig
	networkClient *NetworkClient
	batchSize     int

	closeChan chan bool
}

func NewRecordAccumulator(config *RecordAccumulatorConfig, networkClient *NetworkClient) *RecordAccumulator {
	accumulator := &RecordAccumulator{}
	accumulator.input = make(chan *ProducerRecord, config.batchSize)
	accumulator.config = config
	accumulator.batchSize = config.batchSize
	accumulator.networkClient = networkClient
	accumulator.closeChan = make(chan bool)

	go accumulator.sender()

	return accumulator
}

func (ra *RecordAccumulator) sender() {
	timeout := time.NewTimer(ra.config.linger)
	batch := make([]*ProducerRecord, ra.batchSize)
	batchIndex := 0
	for {
		select {
		case <-ra.closeChan:
			return
		default:
			select {
			case message := <-ra.input:
				{
					batch[batchIndex] = message
					batchIndex++
					if batchIndex >= ra.batchSize {
						ra.networkClient.send(batch[:batchIndex])
						batch = make([]*ProducerRecord, ra.batchSize)
						batchIndex = 0
						timeout.Reset(ra.config.linger)
					}
				}
			case <-timeout.C:
				if batchIndex > 0 {
					ra.networkClient.send(batch[:batchIndex])
					batch = make([]*ProducerRecord, ra.batchSize)
					batchIndex = 0
				}
				timeout.Reset(ra.config.linger)
			}
		}
	}
}
