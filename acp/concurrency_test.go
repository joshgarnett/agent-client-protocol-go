package acp

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/joshgarnett/agent-client-protocol-go/acp/api"

	"github.com/stretchr/testify/suite"
)

// ConcurrencyTestSuite tests concurrent operations and thread safety.
type ConcurrencyTestSuite struct {
	suite.Suite

	pair *ConnectionPair
}

func (s *ConcurrencyTestSuite) SetupTest() {
	s.pair = NewConnectionPair(s.T())
}

func (s *ConcurrencyTestSuite) TearDownTest() {
	if s.pair != nil {
		s.pair.Close()
	}
}

func (s *ConcurrencyTestSuite) TestConcurrentInitialization() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const numGoroutines = 10
	var wg sync.WaitGroup
	results := make(chan *api.InitializeResponse, numGoroutines)
	errors := make(chan error, numGoroutines)

	// Launch multiple initialization attempts concurrently.
	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()

			request := SampleInitializeRequest()
			result, err := s.pair.AgentConn.Initialize(ctx, request)
			if err != nil {
				errors <- err
			} else {
				results <- result
			}
		}()
	}

	wg.Wait()
	close(results)
	close(errors)

	// Verify all requests succeeded.
	resultCount := 0
	for result := range results {
		s.NotNil(result)
		s.True(result.AgentCapabilities.LoadSession)
		resultCount++
	}

	errorCount := 0
	for err := range errors {
		s.T().Errorf("Unexpected error: %v", err)
		errorCount++
	}

	s.Equal(numGoroutines, resultCount)
	s.Equal(0, errorCount)
}

func (s *ConcurrencyTestSuite) TestConcurrentSessionOperations() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Initialize first.
	s.initializeConnection(ctx)

	const numSessions = 50
	var wg sync.WaitGroup
	sessionIDs := make(chan api.SessionId, numSessions)
	errors := make(chan error, numSessions)

	// Create multiple sessions concurrently.
	for i := range numSessions {
		wg.Add(1)
		go func(sessionNum int) {
			defer wg.Done()

			request := SampleNewSessionRequest()
			// Make each session unique by modifying the cwd.
			request.Cwd = request.Cwd + "/" + string(rune('0'+sessionNum))

			result, err := s.pair.AgentConn.SessionNew(ctx, request)
			if err != nil {
				errors <- err
			} else {
				sessionIDs <- result.SessionId
			}
		}(i)
	}

	wg.Wait()
	close(sessionIDs)
	close(errors)

	// Verify all sessions were created.
	receivedIDs := make([]api.SessionId, 0)
	for id := range sessionIDs {
		receivedIDs = append(receivedIDs, id)
	}

	errorCount := 0
	for err := range errors {
		s.T().Errorf("Unexpected error: %v", err)
		errorCount++
	}

	s.Len(receivedIDs, numSessions)
	s.Equal(0, errorCount)

	// Verify all session IDs are unique.
	idMap := make(map[api.SessionId]bool)
	for _, id := range receivedIDs {
		s.False(idMap[id], "Duplicate session ID: %s", id)
		idMap[id] = true
	}

	// Verify sessions were recorded in test agent.
	sessions := s.pair.TestAgent.GetSessions()
	s.Len(sessions, numSessions)
}

func (s *ConcurrencyTestSuite) TestConcurrentFileOperations() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Initialize connection.
	s.initializeConnection(ctx)

	// Set up test files.
	testFiles := make(map[string]string)
	for i := range 20 {
		path := "/test/concurrent_" + string(rune('a'+i)) + ".txt"
		content := "Content for file " + string(rune('a'+i))
		testFiles[path] = content
		s.pair.TestClient.AddFileContent(path, content)
	}

	var wg sync.WaitGroup
	const numConcurrentReads = 100
	readResults := make(chan *api.ReadTextFileResponse, numConcurrentReads)
	readErrors := make(chan error, numConcurrentReads)

	// Perform concurrent file reads.
	for i := range numConcurrentReads {
		wg.Add(1)
		go func(readNum int) {
			defer wg.Done()

			// Pick a random file from testFiles.
			fileIndex := readNum % len(testFiles)
			var filePath string
			count := 0
			for path := range testFiles {
				if count == fileIndex {
					filePath = path
					break
				}
				count++
			}

			request := SampleReadTextFileRequest("session-1", filePath)
			result, err := s.pair.ClientConn.FsReadTextFile(ctx, request)
			if err != nil {
				readErrors <- err
			} else {
				readResults <- result
			}
		}(i)
	}

	wg.Wait()
	close(readResults)
	close(readErrors)

	// Verify all reads succeeded.
	successCount := 0
	for result := range readResults {
		s.NotNil(result)
		s.NotEmpty(result.Content)
		successCount++
	}

	errorCount := 0
	for err := range readErrors {
		s.T().Errorf("Unexpected read error: %v", err)
		errorCount++
	}

	s.Equal(numConcurrentReads, successCount)
	s.Equal(0, errorCount)

	// Now test concurrent writes.
	const numConcurrentWrites = 50
	var writeWg sync.WaitGroup
	writeErrors := make(chan error, numConcurrentWrites)

	for i := range numConcurrentWrites {
		writeWg.Add(1)
		go func(writeNum int) {
			defer writeWg.Done()

			filePath := "/output/concurrent_write_" + string(rune('0'+(writeNum%10))) + ".txt"
			content := "Concurrent write content " + string(rune('0'+writeNum))

			request := SampleWriteTextFileRequest("session-1", filePath, content)
			err := s.pair.ClientConn.FsWriteTextFile(ctx, request)
			if err != nil {
				writeErrors <- err
			}
		}(i)
	}

	writeWg.Wait()
	close(writeErrors)

	// Verify all writes succeeded.
	writeErrorCount := 0
	for err := range writeErrors {
		s.T().Errorf("Unexpected write error: %v", err)
		writeErrorCount++
	}
	s.Equal(0, writeErrorCount)

	// Verify writes were recorded.
	writtenFiles := s.pair.TestClient.GetWrittenFiles()
	s.Len(writtenFiles, numConcurrentWrites)
}

func (s *ConcurrencyTestSuite) TestConcurrentMixedOperations() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Initialize connection.
	s.initializeConnection(ctx)

	var wg sync.WaitGroup
	const numOperations = 30
	operationResults := make(chan string, numOperations)
	operationErrors := make(chan error, numOperations)

	// Mix different types of operations.
	for i := range numOperations {
		wg.Add(1)
		go func(opNum int) {
			defer wg.Done()
			s.executeMixedOperation(ctx, opNum, operationResults, operationErrors)
		}(i)
	}

	wg.Wait()
	close(operationResults)
	close(operationErrors)

	// Verify all operations completed.
	successCount := 0
	for result := range operationResults {
		s.NotEmpty(result)
		successCount++
	}

	errorCount := 0
	for err := range operationErrors {
		s.T().Errorf("Unexpected operation error: %v", err)
		errorCount++
	}

	s.Equal(numOperations, successCount)
	s.Equal(0, errorCount)
}

func (s *ConcurrencyTestSuite) TestConcurrentNotifications() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Initialize and create base session.
	s.initializeConnection(ctx)
	sessionResponse := s.createSession(ctx)

	const numNotifications = 20
	var wg sync.WaitGroup
	notificationErrors := make(chan error, numNotifications)

	// Send concurrent cancel notifications.
	for range numNotifications {
		wg.Add(1)
		go func() {
			defer wg.Done()

			cancelRequest := &api.CancelNotification{
				SessionId: sessionResponse.SessionId,
			}
			err := s.pair.AgentConn.SessionCancel(ctx, cancelRequest)
			if err != nil {
				notificationErrors <- err
			}
		}()
	}

	wg.Wait()
	close(notificationErrors)

	// Verify no errors in sending notifications.
	errorCount := 0
	for err := range notificationErrors {
		s.T().Errorf("Unexpected notification error: %v", err)
		errorCount++
	}
	s.Equal(0, errorCount)

	// Give notifications time to be processed.
	time.Sleep(200 * time.Millisecond)

	// Verify cancellations were received (all should have same session ID).
	cancellations := s.pair.TestAgent.GetCancellationsReceived()
	s.Len(cancellations, numNotifications)
	for _, sessionID := range cancellations {
		s.Equal(string(sessionResponse.SessionId), sessionID)
	}
}

func (s *ConcurrencyTestSuite) TestConcurrentErrorHandling() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Initialize connection.
	s.initializeConnection(ctx)

	// Configure test client to return errors for certain operations.
	s.pair.TestClient.SetShouldError("fs/read_text_file", true)

	const numConcurrentRequests = 30
	var wg sync.WaitGroup
	errorResults := make(chan error, numConcurrentRequests)

	// Make concurrent requests that should all fail.
	for i := range numConcurrentRequests {
		wg.Add(1)
		go func(reqNum int) {
			defer wg.Done()

			request := SampleReadTextFileRequest("session-1", "/error/file_"+string(rune('0'+reqNum))+".txt")
			_, err := s.pair.ClientConn.FsReadTextFile(ctx, request)
			errorResults <- err
		}(i)
	}

	wg.Wait()
	close(errorResults)

	// Verify all requests failed with expected errors.
	errorCount := 0
	for err := range errorResults {
		s.Require().Error(err)
		AssertACPError(s.T(), err, api.ErrorCodeNotFound)
		errorCount++
	}

	s.Equal(numConcurrentRequests, errorCount)
}

func (s *ConcurrencyTestSuite) TestConcurrentConnectionManagement() {
	// Test creating and managing multiple connection pairs concurrently.
	const numConnections = 10
	var wg sync.WaitGroup
	connectionResults := make(chan bool, numConnections)
	connectionErrors := make(chan error, numConnections)

	for i := range numConnections {
		wg.Add(1)
		go func(_ int) {
			defer wg.Done()

			// Create independent connection pair.
			pair := NewConnectionPair(s.T())
			defer pair.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Test basic operation on each connection.
			request := SampleInitializeRequest()
			result, err := pair.AgentConn.Initialize(ctx, request)
			if err != nil {
				connectionErrors <- err
			} else {
				s.NotNil(result)
				connectionResults <- true
			}
		}(i)
	}

	wg.Wait()
	close(connectionResults)
	close(connectionErrors)

	// Verify all connections worked.
	successCount := 0
	for success := range connectionResults {
		s.True(success)
		successCount++
	}

	errorCount := 0
	for err := range connectionErrors {
		s.T().Errorf("Unexpected connection error: %v", err)
		errorCount++
	}

	s.Equal(numConnections, successCount)
	s.Equal(0, errorCount)
}

// Helper methods.

func (s *ConcurrencyTestSuite) initializeConnection(ctx context.Context) *api.InitializeResponse {
	request := SampleInitializeRequest()
	response, err := s.pair.AgentConn.Initialize(ctx, request)
	s.Require().NoError(err)
	s.Require().NotNil(response)
	return response
}

func (s *ConcurrencyTestSuite) createSession(ctx context.Context) *api.NewSessionResponse {
	request := SampleNewSessionRequest()
	response, err := s.pair.AgentConn.SessionNew(ctx, request)
	s.Require().NoError(err)
	s.Require().NotNil(response)
	return response
}

func (s *ConcurrencyTestSuite) executeMixedOperation(
	ctx context.Context,
	opNum int,
	operationResults chan string,
	operationErrors chan error,
) {
	switch opNum % 3 {
	case 0:
		// Session operation.
		request := SampleNewSessionRequest()
		request.Cwd = "/mixed/" + string(rune('0'+opNum))
		result, err := s.pair.AgentConn.SessionNew(ctx, request)
		if err != nil {
			operationErrors <- err
		} else {
			operationResults <- "session:" + string(result.SessionId)
		}

	case 1:
		// File read operation.
		s.pair.TestClient.AddFileContent("/mixed/file_"+string(rune('0'+opNum))+".txt", "mixed content")
		request := SampleReadTextFileRequest("session-1", "/mixed/file_"+string(rune('0'+opNum))+".txt")
		result, err := s.pair.ClientConn.FsReadTextFile(ctx, request)
		if err != nil {
			operationErrors <- err
		} else {
			operationResults <- "read:" + result.Content
		}

	case 2:
		// File write operation.
		request := SampleWriteTextFileRequest(
			"session-1",
			"/mixed/output_"+string(rune('0'+opNum))+".txt",
			"mixed output",
		)
		err := s.pair.ClientConn.FsWriteTextFile(ctx, request)
		if err != nil {
			operationErrors <- err
		} else {
			operationResults <- "write:success"
		}
	}
}

func TestConcurrencyTestSuite(t *testing.T) {
	suite.Run(t, new(ConcurrencyTestSuite))
}
