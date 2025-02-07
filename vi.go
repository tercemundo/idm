package main

import (
        "flag"
        "fmt"
        "io/ioutil"
        "log"
        "math"
        "net/http"
        "os"
        "strings"
        "sync"
        "sync/atomic"
        "time"
)

type VideoResult struct {
        ID        int
        URL       string
        Timestamp string
}

type VimeoScraper struct {
        threads       int
        results       []VideoResult
        mutex         sync.Mutex
        requestCount  int64
        foundCount    int64
        client        *http.Client
        rateLimitWait int64
        currentID     int64
}

func NewVimeoScraper(threads int) *VimeoScraper {
        return &VimeoScraper{
                threads: threads,
                client: &http.Client{
                        Timeout: 10 * time.Second,
                },
                rateLimitWait: 100,
        }
}

func (vs *VimeoScraper) adjustRateLimit(success bool) {
        const (
                minWait = 100   // 100ms mínimo
                maxWait = 5000  // 5 segundos máximo
        )

        vs.mutex.Lock()
        defer vs.mutex.Unlock()

        if success {
                newWait := math.Max(float64(vs.rateLimitWait)*0.8, minWait)
                vs.rateLimitWait = int64(newWait)
        } else {
                newWait := math.Min(float64(vs.rateLimitWait)*2, maxWait)
                vs.rateLimitWait = int64(newWait)
        }
}

func (vs *VimeoScraper) worker(jobs <-chan int, wg *sync.WaitGroup) {
        defer wg.Done()

        for videoID := range jobs {
                atomic.StoreInt64(&vs.currentID, int64(videoID))
                time.Sleep(time.Duration(atomic.LoadInt64(&vs.rateLimitWait)) * time.Millisecond)

                url := fmt.Sprintf("https://player.vimeo.com/video/%d", videoID)
                req, err := http.NewRequest("GET", url, nil)
                if err != nil {
                        continue
                }

                req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

                resp, err := vs.client.Do(req)

                requestCount := atomic.AddInt64(&vs.requestCount, 1)

                // Modificado para imprimir cada 600 interacciones
                if requestCount%600 == 0 {
                        log.Printf("Progreso: %d solicitudes procesadas - ID actual: %d",
                                requestCount, videoID)
                }

                if err != nil {
                        vs.adjustRateLimit(false)
                        continue
                }

                if resp.StatusCode == http.StatusOK {
                        body, err := ioutil.ReadAll(resp.Body)
                        resp.Body.Close()

                        if err != nil {
                                continue
                        }

                        bodyStr := string(body)

                        if strings.Contains(bodyStr, "user177459844") {
                                result := VideoResult{
                                        ID:        videoID,
                                        URL:       fmt.Sprintf("https://vimeo.com/%d", videoID),
                                        Timestamp: time.Now().Format("2006-01-02 15:04:05"),
                                }

                                titleStart := strings.Index(bodyStr, `"title":"`)
                                title := "Sin título"
                                if titleStart != -1 {
                                        titleStart += 9
                                        titleEnd := strings.Index(bodyStr[titleStart:], `"`)
                                        if titleEnd != -1 {
                                                title = bodyStr[titleStart : titleStart+titleEnd]
                                        }
                                }

                                log.Printf("¡VIDEO ENCONTRADO! ID: %d - URL: %s - Título: %s",
                                        result.ID, result.URL, title)

                                vs.mutex.Lock()
                                vs.results = append(vs.results, result)
                                vs.mutex.Unlock()

                                atomic.AddInt64(&vs.foundCount, 1)
                                vs.adjustRateLimit(true)
                        }
                } else {
                        vs.adjustRateLimit(false)
                }
        }
}

func (vs *VimeoScraper) SearchVideos(startID, endID int, outputFile string) {
        fmt.Printf("Iniciando búsqueda de videos entre ID %d y %d\n", startID, endID)
        startTime := time.Now()

        jobs := make(chan int, vs.threads)
        var wg sync.WaitGroup

        for i := 0; i < vs.threads; i++ {
                wg.Add(1)
                go vs.worker(jobs, &wg)
        }

        go func() {
                for id := startID; id <= endID; id++ {
                        jobs <- id
                }
                close(jobs)
        }()

        wg.Wait()

        file, err := os.Create(outputFile)
        if err != nil {
                log.Fatalf("Error creando archivo de salida: %v", err)
        }
        defer file.Close()

        for _, result := range vs.results {
                fmt.Fprintf(file, "ID: %d, URL: %s, Encontrado: %s\n",
                        result.ID, result.URL, result.Timestamp)
        }

        duration := time.Since(startTime)

        fmt.Println("\nInforme Final:")
        fmt.Printf("Videos encontrados: %d\n", atomic.LoadInt64(&vs.foundCount))
        fmt.Printf("Solicitudes realizadas: %d\n", atomic.LoadInt64(&vs.requestCount))
        fmt.Printf("Tiempo total de ejecución: %.2f segundos\n", duration.Seconds())
}

func main() {
        startID := flag.Int("start", 0, "ID inicial")
        endID := flag.Int("end", 0, "ID final")
        threads := flag.Int("threads", 10, "Número de goroutines (default: 10)")
        outputFile := flag.String("output", "resultados.txt", "Archivo de salida (default: resultados.txt)")

        flag.Parse()

        if *startID == 0 || *endID == 0 {
                fmt.Println("Los parámetros --start y --end son requeridos")
                flag.Usage()
                os.Exit(1)
        }

        scraper := NewVimeoScraper(*threads)
        scraper.SearchVideos(*startID, *endID, *outputFile)
}
