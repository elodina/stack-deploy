/* Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License.  You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License. */

package producer

import (
	"github.com/elodina/siesta"
	"io"
	"net"
	"time"
)

type ConnectionRequest struct {
	connection *net.TCPConn
	request    *NetworkRequest
}

//TODO proper config entry names that match upstream Kafka
type SelectorConfig struct {
	ClientID        string
	MaxRequests     int
	SendRoutines    int
	ReceiveRoutines int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	RequiredAcks    int
}

func DefaultSelectorConfig() *SelectorConfig {
	return &SelectorConfig{
		ClientID:        "siesta",
		MaxRequests:     10,
		SendRoutines:    10,
		ReceiveRoutines: 10,
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    5 * time.Second,
		RequiredAcks:    1,
	}
}

func NewSelectorConfig(producerConfig *ProducerConfig) *SelectorConfig {
	return &SelectorConfig{
		ClientID:        producerConfig.ClientID,
		MaxRequests:     producerConfig.MaxRequests,
		SendRoutines:    producerConfig.SendRoutines,
		ReceiveRoutines: producerConfig.ReceiveRoutines,
		ReadTimeout:     producerConfig.ReadTimeout,
		WriteTimeout:    producerConfig.WriteTimeout,
		RequiredAcks:    producerConfig.RequiredAcks,
	}
}

type Selector struct {
	config         *SelectorConfig
	correlationIDs *siesta.CorrelationIDGenerator
	requests       chan *NetworkRequest
	responses      chan *ConnectionRequest
}

func NewSelector(config *SelectorConfig) *Selector {
	selector := &Selector{
		config:         config,
		correlationIDs: new(siesta.CorrelationIDGenerator),
		requests:       make(chan *NetworkRequest, config.MaxRequests),
		responses:      make(chan *ConnectionRequest, config.MaxRequests),
	}
	selector.Start()
	return selector
}

func (s *Selector) Start() {
	for i := 0; i < s.config.SendRoutines; i++ {
		go s.requestDispatcher()
	}

	if s.config.RequiredAcks > 0 {
		for i := 0; i < s.config.ReceiveRoutines; i++ {
			go s.responseDispatcher()
		}
	}
}

func (s *Selector) Close() {
	close(s.requests)
	close(s.responses)
}

func (s *Selector) Send(connection *siesta.BrokerConnection, request siesta.Request) <-chan *rawResponseAndError {
	responseChan := make(chan *rawResponseAndError, 1) //make this buffered so we don't block if noone reads the response
	s.requests <- &NetworkRequest{connection, request, responseChan}

	return responseChan
}

func (s *Selector) requestDispatcher() {
	for request := range s.requests {
		connection := request.connection
		id := s.correlationIDs.NextCorrelationID()
		conn, err := connection.GetConnection()
		if err != nil {
			request.responseChan <- &rawResponseAndError{nil, connection, err, nil}
			continue
		}

		if err := s.send(id, conn, request.request); err != nil {
			request.responseChan <- &rawResponseAndError{nil, connection, err, nil}
			connection.ReleaseConnection(conn)
			continue
		}

		if s.config.RequiredAcks > 0 {
			s.responses <- &ConnectionRequest{connection: conn, request: request}
		} else {
			request.responseChan <- &rawResponseAndError{nil, connection, nil, nil}
			connection.ReleaseConnection(conn)
		}
	}
}

func (s *Selector) responseDispatcher() {
	for connectionResponse := range s.responses {
		connection := connectionResponse.request.connection
		conn := connectionResponse.connection
		responseChan := connectionResponse.request.responseChan

		bytes, err := s.receive(conn)
		if err != nil {
			responseChan <- &rawResponseAndError{nil, connection, nil, err}
			connection.ReleaseConnection(conn)
			continue
		}

		connection.ReleaseConnection(conn)
		responseChan <- &rawResponseAndError{bytes, connection, nil, err}
	}
}

func (s *Selector) send(correlationID int32, conn *net.TCPConn, request siesta.Request) error {
	writer := siesta.NewRequestHeader(correlationID, s.config.ClientID, request)
	bytes := make([]byte, writer.Size())
	encoder := siesta.NewBinaryEncoder(bytes)
	writer.Write(encoder)

	err := conn.SetWriteDeadline(time.Now().Add(s.config.WriteTimeout))
	if err != nil {
		return err
	}
	_, err = conn.Write(bytes)
	return err
}

func (s *Selector) receive(conn *net.TCPConn) ([]byte, error) {
	err := conn.SetReadDeadline(time.Now().Add(s.config.ReadTimeout))
	if err != nil {
		return nil, err
	}
	header := make([]byte, 8)
	_, err = io.ReadFull(conn, header)
	if err != nil {
		return nil, err
	}

	decoder := siesta.NewBinaryDecoder(header)
	length, err := decoder.GetInt32()
	if err != nil {
		return nil, err
	}
	response := make([]byte, length-4)
	_, err = io.ReadFull(conn, response)
	if err != nil {
		return nil, err
	}

	return response, nil
}

//TODO better struct name
type NetworkRequest struct {
	connection   *siesta.BrokerConnection
	request      siesta.Request
	responseChan chan *rawResponseAndError
}

type rawResponseAndError struct {
	bytes      []byte
	connection *siesta.BrokerConnection
	sendErr    error
	receiveErr error
}
