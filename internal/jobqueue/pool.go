package jobqueue

type Pool struct {
	jobs chan func()
}

func NewPool(worker, queueSize int) *Pool {
	p := &Pool{jobs: make(chan func(), queueSize)}
	for range worker {
		go p.worker()
	}

	return p
}

func (p *Pool) worker() {
	for job := range p.jobs {
		job()
	}
}

func (p *Pool) Submit(job func()) {
	p.jobs <- job
}

func (p *Pool) SubmitWait(job func()) {
	done := make(chan bool)

	p.jobs <- func() {
		job()
		close(done)
	}
	<-done
}
