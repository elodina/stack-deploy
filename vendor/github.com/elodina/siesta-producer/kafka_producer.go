package producer

import (
	"fmt"
	"log"

	"github.com/elodina/siesta"
)

type ProducerRecord struct {
	Topic     string
	Partition int32
	Key       interface{}
	Value     interface{}

	encodedKey   []byte
	encodedValue []byte
	metadataChan chan *RecordMetadata
}

type RecordMetadata struct {
	Record    *ProducerRecord
	Offset    int64
	Topic     string
	Partition int32
	Error     error
}

type Serializer func(interface{}) ([]byte, error)

func ByteSerializer(value interface{}) ([]byte, error) {
	if value == nil {
		return nil, nil
	}

	if array, ok := value.([]byte); ok {
		return array, nil
	}

	return nil, fmt.Errorf("Can't serialize %v", value)
}

func StringSerializer(value interface{}) ([]byte, error) {
	if str, ok := value.(string); ok {
		return []byte(str), nil
	}

	return nil, fmt.Errorf("Can't serialize %v to string", value)
}

type Producer interface {
	// Send the given record asynchronously and return a channel which will eventually contain the response information.
	Send(*ProducerRecord) <-chan *RecordMetadata

	// Tries to close the producer cleanly.
	Close()
}

type KafkaProducer struct {
	config          *ProducerConfig
	keySerializer   Serializer
	valueSerializer Serializer
	messagesChan    chan *ProducerRecord
	connector       siesta.Connector
	selector        *Selector
}

func NewKafkaProducer(config *ProducerConfig, keySerializer Serializer, valueSerializer Serializer, connector siesta.Connector) *KafkaProducer {
	log.Println("Starting the Kafka producer")
	producer := &KafkaProducer{}
	producer.config = config
	producer.messagesChan = make(chan *ProducerRecord, config.BatchSize)
	producer.keySerializer = keySerializer
	producer.valueSerializer = valueSerializer
	producer.connector = connector

	selectorConfig := NewSelectorConfig(config)
	producer.selector = NewSelector(selectorConfig)

	go producer.messageDispatchLoop()

	log.Println("Kafka producer started")

	return producer
}

func (kp *KafkaProducer) Send(record *ProducerRecord) <-chan *RecordMetadata {
	record.metadataChan = make(chan *RecordMetadata, 1)
	kp.send(record)
	return record.metadataChan
}

func (kp *KafkaProducer) send(record *ProducerRecord) {
	metadataChan := record.metadataChan
	metadata := new(RecordMetadata)

	serializedKey, err := kp.keySerializer(record.Key)
	if err != nil {
		metadata.Error = err
		metadataChan <- metadata
		return
	}

	serializedValue, err := kp.valueSerializer(record.Value)
	if err != nil {
		metadata.Error = err
		metadataChan <- metadata
		return
	}

	record.encodedKey = serializedKey
	record.encodedValue = serializedValue

	partitions, err := kp.connector.Metadata().PartitionsFor(record.Topic)
	if err != nil {
		metadata.Error = err
		metadataChan <- metadata
		return
	}

	partition, err := kp.config.Partitioner.Partition(record, partitions)
	if err != nil {
		metadata.Error = err
		metadataChan <- metadata
		return
	}
	record.Partition = partition

	kp.messagesChan <- record
}

func (kp *KafkaProducer) messageDispatchLoop() {
	accumulators := make(map[string]chan *ProducerRecord)
	for message := range kp.messagesChan {
		accumulator := accumulators[message.Topic]
		if accumulator == nil {
			accumulator = make(chan *ProducerRecord, kp.config.BatchSize)
			accumulators[message.Topic] = accumulator
			go kp.topicDispatchLoop(accumulator)
		}

		accumulator <- message
	}

	for _, accumulator := range accumulators {
		close(accumulator)
	}
}

func (kp *KafkaProducer) topicDispatchLoop(topicMessagesChan chan *ProducerRecord) {
	accumulators := make(map[int32]*RecordAccumulator)
	for message := range topicMessagesChan {
		accumulator := accumulators[message.Partition]
		if accumulator == nil {
			networkClientConfig := &NetworkClientConfig{
				RequiredAcks: kp.config.RequiredAcks,
				AckTimeoutMs: kp.config.AckTimeoutMs,
				Retries:      kp.config.Retries,
				RetryBackoff: kp.config.RetryBackoff,
				Topic:        message.Topic,
				Partition:    message.Partition,
			}
			networkClient := NewNetworkClient(networkClientConfig, kp.connector, kp.selector)

			accumulatorConfig := &RecordAccumulatorConfig{
				batchSize: kp.config.BatchSize,
				linger:    kp.config.Linger,
			}
			accumulator = NewRecordAccumulator(accumulatorConfig, networkClient)
			accumulators[message.Partition] = accumulator
		}

		accumulator.input <- message
	}

	for _, accumulator := range accumulators {
		close(accumulator.closeChan)
	}
}

func (kp *KafkaProducer) Close() {
	close(kp.messagesChan)
}
