package goworker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

type poller struct {
	process
	isStrict bool
}

func newPoller(queues []string, isStrict bool) (*poller, error) {
	process, err := newProcess("poller", queues)
	if err != nil {
		return nil, err
	}
	return &poller{
		process:  *process,
		isStrict: isStrict,
	}, nil
}

func (p *poller) getJobStatus(conn *RedisConn, job *Job) *JobStatus {
	reply, err := conn.Do("GET", fmt.Sprintf("%sjob:%s:status", workerSettings.Namespace, job.Payload.UUID))
	if err != nil || reply == nil {
		logger.Debugf("Got error while fetching job (%s) status: %v", job.Payload.UUID, err);
		return nil
	}

	jobStatus := &JobStatus{}
	decoder := json.NewDecoder(bytes.NewReader(reply.([]byte)))
	// we dont care about status here
	if err = decoder.Decode(jobStatus); err != nil {
		logger.Debugf("Got error while decoding job (%s) status: %v", job.Payload.UUID, err);
		return nil
	}
	logger.Debugf("Got job (%s) status: %v", job.Payload.UUID, jobStatus);

	return jobStatus
}

func (p *poller) getJob(conn *RedisConn) (*Job, error) {
	for _, queue := range p.queues(p.isStrict) {
		logger.Debugf("Checking %s", queue)

		reply, err := conn.Do("LPOP", fmt.Sprintf("%squeue:%s", workerSettings.Namespace, queue))
		if err != nil {
			return nil, err
		}
		if reply == nil {
			return nil, nil
		}

		logger.Debugf("Found job on %s", queue)

		job := &Job{Queue: queue}

		decoder := json.NewDecoder(bytes.NewReader(reply.([]byte)))
		if workerSettings.UseNumber {
			decoder.UseNumber()
		}

		if err := decoder.Decode(&job.Payload); err != nil {
			return nil, err
		}

		//try to get job status
		count := 0
		for job.Status == nil && count < 5 {
			time.Sleep(100 * time.Millisecond)
			count++
			job.Status = p.getJobStatus(conn, job)
		}

		return job, nil
	}

	return nil, nil
}

func (p *poller) poll(interval time.Duration, quit <-chan bool) (<-chan *Job, error) {
	jobs := make(chan *Job)

	conn, err := GetConn()
	if err != nil {
		logger.Criticalf("Error on getting connection in poller %s: %v", p, err)
		close(jobs)
		return nil, err
	} else {
		p.open(conn)
		p.start(conn)
		PutConn(conn)
	}

	go func() {
		defer func() {
			close(jobs)

			conn, err := GetConn()
			if err != nil {
				logger.Criticalf("Error on getting connection in poller %s: %v", p, err)
				return
			} else {
				p.finish(conn)
				p.close(conn)
				PutConn(conn)
			}
		}()

		for {
			select {
			case <-quit:
				return
			default:
				conn, err := GetConn()
				if err != nil {
					logger.Criticalf("Error on getting connection in poller %s: %v", p, err)
					return
				}

				job, err := p.getJob(conn)
				if err != nil {
					logger.Criticalf("Error on %v getting job from %v: %v", p, p.Queues, err)
					PutConn(conn)
					return
				}
				if job != nil {
					conn.Send("INCR", fmt.Sprintf("%sstat:processed:%v", workerSettings.Namespace, p))
					conn.Flush()
					PutConn(conn)
					select {
					case jobs <- job:
					case <-quit:
						buf, err := json.Marshal(job.Payload)
						if err != nil {
							logger.Criticalf("Error requeueing %v: %v", job, err)
							return
						}
						conn, err := GetConn()
						if err != nil {
							logger.Criticalf("Error on getting connection in poller %s: %v", p, err)
							return
						}

						conn.Send("LPUSH", fmt.Sprintf("%squeue:%s", workerSettings.Namespace, job.Queue), buf)
						conn.Flush()
						PutConn(conn)
						return
					}
				} else {
					PutConn(conn)
					if workerSettings.ExitOnComplete {
						return
					}
					logger.Debugf("Sleeping for %v", interval)
					logger.Debugf("Waiting for %v", p.Queues)

					timeout := time.After(interval)
					select {
					case <-quit:
						return
					case <-timeout:
					}
				}
			}
		}
	}()

	return jobs, nil
}
