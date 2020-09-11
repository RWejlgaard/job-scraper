package companies

import (
	"encoding/json"
	"io/ioutil"
	jobs "job-scraper/internal"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

// https://careers.google.com/api/v2/jobs/search/?company=Google&company=Google%20Fiber&company=YouTube&employment_type=FULL_TIME&hl=en_US&jlo=en_US&location=London%2C%20UK&q=&sort_by=relevance
// https://careers.google.com/api/v2/jobs/get/?job_name=jobs%2F136853555093873350

type googleAPI struct {
	Count    int `json:"count"`
	NextPage int `json:"next_page"`
	Jobs     []struct {
		Description string   `json:"description"`
		Location    []string `json:"locations"`
		// String of LI elements
		Summary  string `json:"summary"`
		JobTitle string `json:"job_title"`
		JobID    string `json:"job_id"`
	} `json:"jobs"`
}

type googleJob struct {
	Title        string   `json:"title"`
	Requirements string   `json:"qualifications"`
	Education    []string `json:"education_levels"`
}

type Google jobs.JobSource

func (g Google) GetJobs(logger *zap.Logger) []jobs.Job {
	if len(g.Jobs) == 0 {
		sugar := logger.Sugar()
		sugar.Info("Jobs have not previously been found, finding jobs.")
		g.findJobs(logger)
	}
	return g.Jobs
}

func (g Google) GetURL() string {
	return g.URL
}

func (g Google) GetPath() string {
	return g.FilePath
}

func (g *Google) findJobs(logger *zap.Logger) {
	var gAPI googleAPI
	url := g.GetURL()
	pageNum := 1
	sugar := logger.Sugar()
	jobs := []jobs.Job{}

	for {

		pagnatedURL := url + "&page=" + strconv.Itoa(pageNum)
		sugar.Infof("Querying Google API for jobs %v", pagnatedURL)

		resp, err := http.Get(pagnatedURL)

		responseData, err := ioutil.ReadAll(resp.Body)
		err = json.Unmarshal(responseData, &gAPI)

		jobSet, err := g.gatherSpecs(gAPI, logger)

		jobs = append(jobs, jobSet...)

		if err != nil {
			sugar.Error(zap.Error(err))
		}

		if len(gAPI.Jobs) == 0 {
			break
		}

		pageNum++

	}

	g.Jobs = jobs

}

func (g Google) gatherSpecs(gAPI googleAPI, logger *zap.Logger) ([]jobs.Job, error) {
	var gJob googleJob
	sugar := logger.Sugar()
	foundJobs := []jobs.Job{}
	re := regexp.MustCompile(`<li.*?>(.*)</li>`)

	// Need to then go the API and get the job spec.
	// https://careers.google.com/api/v2/jobs/get/?job_name=jobs%2F136853555093873350

	for _, item := range gAPI.Jobs {
		job := jobs.Job{}
		url := "https://careers.google.com/api/v2/jobs/get/?job_name=jobs%2F" + strings.Split(item.JobID, "/")[1]
		sugar.Infof("Querying Google API for job %v", url)

		resp, err := http.Get(url)

		if err != nil {
			sugar.Error(zap.Error(err))
		}
		responseData, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			sugar.Error(zap.Error(err))
		}
		err = json.Unmarshal(responseData, &gJob)

		if err != nil {
			sugar.Error(zap.Error(err))
		}

		r := strings.NewReplacer("<p>Minimum qualifications:</p>", "",
			"<ul>", "",
			"</ul>", "",
			"\n", "",
			"<p>Preferred qualifications:</p>", "",
			"<br>", "",
			"<li>", "",
			"</li>", "")

		req := re.FindAllStringSubmatch(gJob.Requirements, -1)

		for _, i := range req {
			for _, req := range i {
				job.Requirements = append(job.Requirements, r.Replace(req))
			}
		}

		job.Title = gJob.Title
		job.Type = "Permanent"

		// jobID is of format "jobs/<jobID>"
		job.URL = "https://careers.google.com/jobs/results/" + strings.Split(item.JobID, "/")[1]

		job.Salary = "N/A"
		job.Location = strings.Join(item.Location, ",")

		foundJobs = append(foundJobs, job)

	}

	return foundJobs, nil
}
