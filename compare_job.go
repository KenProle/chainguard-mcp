package main

import (
	"context"
	"strings"
	"sync"
	"time"
)

const (
	// compareJobTimeout bounds one comparison job.
	compareJobTimeout = 5 * time.Minute
	// compareJobIdle is how long a job may go unpolled before it is dropped.
	compareJobIdle = 5 * time.Minute
	// maxCompareJobs bounds the jobs held at once.
	maxCompareJobs = 32
)

// CompareJobStatus is the state of a comparison running in the background.
// While Status is "running", Checked of Total packages have been looked up
// (Total is 0 until the image's packages are known) and Step says what is
// happening; when it is "done", the comparison's fields are set too.
type CompareJobStatus struct {
	Status  string `json:"status" jsonschema:"'running' or 'done'"`
	Step    string `json:"step,omitempty" jsonschema:"while running: 'packages' (looking up packages on OSV.dev) or 'advisories' (reading Chainguard's advisory feed)"`
	Checked int    `json:"checked"`
	Total   int    `json:"total"`
	*UpstreamComparison
}

type compareJob struct {
	cancel context.CancelFunc

	mu         sync.Mutex
	done       bool
	step       string
	checked    int
	total      int
	result     *UpstreamComparison
	err        error
	lastPolled time.Time
}

// compareJobs runs comparisons in the background, one per reference and
// architecture, for clients that poll for progress.
type compareJobs struct {
	mu   sync.Mutex
	jobs map[string]*compareJob
}

// CompareUpstreamJob starts the comparison, or joins the one already running
// or finished for the same reference and architecture, and returns its
// current state at once. Invalid input fails immediately. A job's error is
// returned once, by the poll that finds it, and the job is then forgotten so
// the next call retries.
func (s *Service) CompareUpstreamJob(in CompareUpstreamInput) (*CompareJobStatus, error) {
	if _, _, _, err := validateCompareInput(in); err != nil {
		return nil, err
	}
	arch, _ := normalizeArch(in.Arch)
	key := strings.ToLower(strings.TrimSpace(in.Image)) + "|" + arch

	s.jobs.mu.Lock()
	s.jobs.dropStale()
	job := s.jobs.jobs[key]
	if job == nil {
		if len(s.jobs.jobs) >= maxCompareJobs {
			s.jobs.mu.Unlock()
			return nil, inputErrorf("too many comparisons are running; try again in a minute")
		}
		ctx, cancel := context.WithTimeout(context.Background(), compareJobTimeout)
		job = &compareJob{cancel: cancel, lastPolled: time.Now()}
		if s.jobs.jobs == nil {
			s.jobs.jobs = make(map[string]*compareJob)
		}
		s.jobs.jobs[key] = job
		go func() {
			defer cancel()
			result, err := s.compareUpstream(ctx, in, func(step string, done, total int) {
				job.mu.Lock()
				job.step, job.checked, job.total = step, done, total
				job.mu.Unlock()
			})
			job.mu.Lock()
			job.done, job.result, job.err = true, result, err
			job.mu.Unlock()
		}()
	}
	s.jobs.mu.Unlock()

	job.mu.Lock()
	defer job.mu.Unlock()
	job.lastPolled = time.Now()
	if !job.done {
		return &CompareJobStatus{Status: "running", Step: job.step, Checked: job.checked, Total: job.total}, nil
	}
	if job.err != nil {
		s.jobs.mu.Lock()
		if s.jobs.jobs[key] == job {
			delete(s.jobs.jobs, key)
		}
		s.jobs.mu.Unlock()
		return nil, job.err
	}
	return &CompareJobStatus{Status: "done", Checked: job.checked, Total: job.total, UpstreamComparison: job.result}, nil
}

// dropStale cancels and forgets jobs nobody has polled for a while. The
// caller holds j.mu.
func (j *compareJobs) dropStale() {
	for key, job := range j.jobs {
		job.mu.Lock()
		idle := time.Since(job.lastPolled) > compareJobIdle
		job.mu.Unlock()
		if idle {
			job.cancel()
			delete(j.jobs, key)
		}
	}
}
