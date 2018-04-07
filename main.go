package main

import (
	"io/ioutil"
	"io"
	"log"
	"net/url"
	"net/http"
	"sync"
	"os"
	"path"
	"fmt"
	"gopkg.in/yaml.v2"
	"github.com/mholt/archiver"
	"github.com/vbauerster/mpb"
	"github.com/vbauerster/mpb/decor"

	"github.com/sbaildon/wow-addon-downloader/providers"
	_ "github.com/sbaildon/wow-addon-downloader/providers/curseforge"
)

type yamlurl struct {
	*url.URL
}

func (j *yamlurl) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	err := unmarshal(&s)
	if err != nil {
		return err
	}
	url, err := url.Parse(s)
	j.URL = url
	return err
}

func download(provider providers.Provider, u url.URL, config config, bar *mpb.Bar, wg *sync.WaitGroup) {
	defer wg.Done()

	_, err := provider.GetName(u)
	if err != nil {
		log.Println(err)
		return
	}

	_, err = provider.GetVersion(u)
	if err != nil {
		log.Println(err)
		return
	}

	var downloadURL = provider.DownloadURL(u)
	bar.Increment()
	resp, err := http.Get(downloadURL)
	if err != nil {
		log.Println(err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Println("Request failed, status not ok")
		return
	}

	/* Create a temporary directory for saving files */
	dir, err := ioutil.TempDir("", "wow-addon-downloader")
	if err != nil {
		log.Println(err)
		return
	}
	defer os.RemoveAll(dir)

	/* Save zip to temporary directory */
	bar.Increment()
	out, err := os.Create(path.Join(dir, path.Base(resp.Request.URL.String())))
	if err != nil {
		log.Println(err)
		return
	}
	defer out.Close()
	io.Copy(out, resp.Body)

	/* Unzip files */
	bar.Increment()
	err = archiver.Zip.Open(out.Name(), config.System.AddonDir)
	if err != nil {
		log.Println(err)
		return
	}

	bar.Increment()
}

type config struct {
	System struct {
		AddonDir string `yaml:"addon_dir"`
	} `yaml:"system"`
	AddOns []yamlurl `yaml:"addons"`
}

func main() {
	configSource, err := ioutil.ReadFile("./config.yml")
	if err != nil {
		log.Fatal("Problem reading config file")
	}

	var config config
	err = yaml.Unmarshal(configSource, &config)
	if err != nil {
		log.Fatal("Can't understand config file. Is it malformed?")
	}

	var wg sync.WaitGroup
	pool := mpb.New(mpb.WithWaitGroup(&wg))

	var steps = []string{"checking version", "downloading", "saving", "unzipping", "finished"}

	for _, url := range config.AddOns {
		provider, err :=  providers.GetProvider(url.URL.Hostname())

		if err != nil {
			bar := pool.AddBar(0,
				mpb.AppendDecorators(decor.StaticName("-----", 5, 0),),
				mpb.PrependDecorators(decor.StaticName(fmt.Sprintf("%s:", url.String()), 0, decor.DSyncSpace+decor.DidentRight)),
				mpb.PrependDecorators(decor.DynamicName(func(s *decor.Statistics) string {
					return fmt.Sprintf("%s", "unsupported")
				}, 16, 1)),
				mpb.PrependDecorators(decor.Elapsed(3, decor.DSyncSpace)),
			)
			bar.Complete()
			continue
		}

		bar := pool.AddBar(int64(len(steps)-1),
			mpb.AppendDecorators(decor.Percentage(5,0),),
			mpb.PrependDecorators(decor.StaticName(fmt.Sprintf("%s:", url.String()), 0, decor.DSyncSpace+decor.DidentRight)),
			mpb.PrependDecorators(decor.DynamicName(func(s *decor.Statistics) string {
				return fmt.Sprintf("%s", steps[s.Current])
			}, 16, 1)),
			mpb.PrependDecorators(decor.Elapsed(3, decor.DSyncSpace)),
		)

		wg.Add(1)
		go download(provider, *url.URL, config, bar, &wg)
	}

	pool.Wait()
	fmt.Println("done")
}