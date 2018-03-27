package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/darrenmce/goci"
	"github.com/stretchr/testify/mock"
	"github.com/darrenmce/goci/docker"
	"errors"
)

type MockDockerLib struct {
	mock.Mock
}

func (m *MockDockerLib) RunContainer(container docker.BuildContainer, command string) (int, error) {
	args := m.Called(container, command)
	return args.Int(0), args.Error(1)
}

var _ = Describe("Goci", func() {

	var (
		jr JobRun
	)

	BeforeEach(func() {
		jr = JobRun{
			InstructionFile: "test.yaml",
			BuildId:         "testBuild1",
			WorkDir:         "/tmp/dir1234",
			Job: Job{
				Name: "test_job",
				Git:  JobGit{
					Repo:"https://gitplace.com/myrepo.git",
				},
				Build: JobBuild{
					Image: "myrepo/myimage",
					Steps: []string{
						"echo 123",
						"npm install",
					},
				},
				Publish: JobPublish{
					Repo: "myrepo/mypublish",
					Registry: "myregistry.com",
					AuthRef: "123",
				},
			},
		}
	})

	Describe("JobRun - RunBuild", func() {
		Context("Successful run", func() {
			var (
				code int
				err error
				dkr *MockDockerLib
			)

			BeforeEach(func() {
				dkr = new(MockDockerLib)
				dkr.On("RunContainer", mock.Anything, mock.Anything).Return(0, nil).Twice()
				code, err = jr.RunBuild(dkr, "123")
			})

			It("should run all the build steps", func() {
				dkr.AssertExpectations(GinkgoT())
			})

			It("should return the zero status code without an error", func () {
				Expect(err).To(BeNil())
				Expect(code).To(Equal(0))
			})

			It("should call with the correct container", func() {
				container := dkr.Calls[0].Arguments.Get(0).(docker.BuildContainer)
				Expect(container.Name).To(Equal("test_job"))
				Expect(container.Image).To(Equal("myrepo/myimage"))
				Expect(container.WorkDir).To(Equal("/build"))
				Expect(container.BuildId).To(Equal("123"))
				Expect(container.Volumes[0].Source).To(Equal("/tmp/dir1234"))
				Expect(container.Volumes[0].Target).To(Equal("/build"))
			})
		})

		Context("Unsuccessful run - exit code", func() {
			var (
				code int
				err error
				dkr *MockDockerLib
			)

			BeforeEach(func() {
				dkr = new(MockDockerLib)
				dkr.On("RunContainer", mock.Anything, mock.Anything).Return(0, nil).Once()
				dkr.On("RunContainer", mock.Anything, mock.Anything).Return(1, nil).Once()
				code, err = jr.RunBuild(dkr, "123")
			})

			It("pass non-zero exit code on", func() {
				dkr.AssertExpectations(GinkgoT())
				Expect(err).To(BeNil())
				Expect(code).To(Equal(1))
			})
		})

		Context("Unsuccessful run - exit code - short circuit", func() {
			var (
				code int
				err error
				dkr *MockDockerLib
			)

			BeforeEach(func() {
				dkr = new(MockDockerLib)
				dkr.On("RunContainer", mock.Anything, mock.Anything).Return(1, nil).Once()
				code, err = jr.RunBuild(dkr, "123")
			})

			It("short circuit on non-zero exit code", func() {
				dkr.AssertExpectations(GinkgoT())
				Expect(err).To(BeNil())
				Expect(code).To(Equal(1))
			})
		})

		Context("Unsuccessful run - exit code", func() {
			var (
				code int
				err error
				dkrError error
				dkr *MockDockerLib
			)

			BeforeEach(func() {
				dkr = new(MockDockerLib)
				dkrError = errors.New("Oops!")
				dkr.On("RunContainer", mock.Anything, mock.Anything).Return(-1, dkrError).Once()
				code, err = jr.RunBuild(dkr, "123")
			})

			It("should pass docker errors on (and short circuit)", func() {
				dkr.AssertExpectations(GinkgoT())
				Expect(err).To(Equal(dkrError))
				Expect(code).To(Equal(-1))
			})
		})
	})
})
