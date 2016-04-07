package producer

import (
	"github.com/elodina/siesta"
	"time"
)

type NetworkClientConfig struct {
	RequiredAcks int
	AckTimeoutMs int32
	Retries      int
	RetryBackoff time.Duration
	Topic        string
	Partition    int32
}

type NetworkClient struct {
	*NetworkClientConfig
	connector siesta.Connector
	selector  *Selector
}

func NewNetworkClient(config *NetworkClientConfig, connector siesta.Connector, selector *Selector) *NetworkClient {
	return &NetworkClient{
		NetworkClientConfig: config,
		connector:           connector,
		selector:            selector,
	}
}

func (nc *NetworkClient) send(batch []*ProducerRecord) {
	if len(batch) == 0 {
		Logger.Warn("NetworkClient received an empty batch?")
		return
	}

	request := nc.buildProduceRequest(batch)

	err := nc.trySend(request, batch)
	if err == nil {
		return
	}

	for i := 0; i < nc.Retries; i++ {
		err = nc.trySend(request, batch)
		if err == nil {
			return
		}

		time.Sleep(nc.RetryBackoff)
	}

	for _, record := range batch {
		record.metadataChan <- &RecordMetadata{Record: record, Error: err}
	}
}

func (nc *NetworkClient) trySend(request *siesta.ProduceRequest, batch []*ProducerRecord) error {
	leader, err := nc.connector.GetLeader(nc.Topic, nc.Partition)
	if err != nil {
		return err
	}

	response := <-nc.selector.Send(leader, request)
	if response.sendErr != nil {
		err = nc.connector.Metadata().Refresh([]string{nc.Topic})
		if err != nil {
			Logger.Warn("Send error occurred, returning it but also failed to refresh metadata: %s", err)
		}
		return response.sendErr
	}

	if nc.RequiredAcks == 0 {
		// acks = 0 case, just complete all requests
		for _, record := range batch {
			record.metadataChan <- &RecordMetadata{
				Record:    record,
				Offset:    -1,
				Topic:     nc.Topic,
				Partition: nc.Partition,
				Error:     siesta.ErrNoError,
			}
		}
		return nil
	}

	if response.receiveErr != nil {
		err = nc.connector.Metadata().Refresh([]string{nc.Topic})
		if err != nil {
			Logger.Warn("Receive error occurred, returning it but also failed to refresh metadata: %s", err)
		}
		return response.receiveErr
	}

	decoder := siesta.NewBinaryDecoder(response.bytes)
	produceResponse := new(siesta.ProduceResponse)
	decodingErr := produceResponse.Read(decoder)
	if decodingErr != nil {
		return decodingErr.Error()
	}

	status, exists := produceResponse.Status[nc.Topic][nc.Partition]
	if exists {
		if status.Error == siesta.ErrNotLeaderForPartition {
			err = nc.connector.Metadata().Refresh([]string{nc.Topic})
			if err != nil {
				Logger.Warn("Produce error occurred, returning it but also failed to refresh metadata: %s", err)
			}
			return status.Error
		}

		currentOffset := status.Offset
		for _, record := range batch {
			record.metadataChan <- &RecordMetadata{
				Record:    record,
				Topic:     nc.Topic,
				Partition: nc.Partition,
				Offset:    currentOffset,
				Error:     status.Error,
			}
			currentOffset++
		}
	}

	return nil
}

func (nc *NetworkClient) buildProduceRequest(batch []*ProducerRecord) *siesta.ProduceRequest {
	request := new(siesta.ProduceRequest)
	request.RequiredAcks = int16(nc.RequiredAcks)
	request.AckTimeoutMs = nc.AckTimeoutMs
	for _, record := range batch {
		request.AddMessage(record.Topic, record.Partition, &siesta.Message{Key: record.encodedKey, Value: record.encodedValue})
	}

	return request
}

func (nc *NetworkClient) close() {
	nc.selector.Close()
}
