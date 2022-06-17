# Go-pdf2image - convert PDF to images with progress
# pico - convert PDF to images with progress

A Go implementation for [@Belval](https://github.com/Belval)'s Python [pdf2image](https://github.com/Belval/pdf2image) but with progress support.

## Install and dependency

```
$ go get github.com/DeathKing/go-pdf2image
```

### Windows

Windows users will have to build or download poppler for Windows. I recommend [@oschwartz10612 version](https://github.com/oschwartz10612/poppler-windows/releases/) which is the most up-to-date. You will then have to add the `bin/` folder to [PATH](https://www.architectryan.com/2018/03/17/add-to-the-path-on-windows-10/) or use `WithPopperPath("C:\path\to\poppler-xx\bin")`.

### Mac

Mac users will have to install [poppler for Mac](http://macappstore.org/poppler/).

### Linux

Most distros ship with `pdftoppm` and `pdftocairo`. If they are not installed, refer to your package manager to install `poppler-utils`

## Usage

```go
import p2i "github.com/DeathKing/go-pdf2image"

func main() {

    // Case 1. Silently convert file with single worker
    task, _ := p2i.Convert("path/to/pdf")
    task.Wait()

    // Case 2. Convert file with multiple worker, instead of `Wait()` for final
    //         result, we take the per-page conversion result immediately
    task, _ = gopdf2image.Convert("path/to/pdf",
        p2i.WithWorkerCount(4),
        p2i.WithProgress(),
    )

    for entry := range task.EntryChan {
        fmt.Printf("[%d/%d] worker#%d converted page %s as file %s \n",
            task.Converted,
            task.Total,
            entry[3], // worker id
            entry[0], // current page
            entry[2], // output filename
        )
    }

    // Case 3. A more fancy usage
    task, _ = p2i.Convert("path/to/pdf",
        p2i.WithPopperPath("path/to/poppler"),
        p2i.WithFormat("jpg"),
        p2i.WithDPI(72),
        p2i.WithPageRange(22, 42),             // Convert from Page 22 to Page 42 (included)
        p2i.WithProgress(),                    // We'd like to know the progress
        p2i.WithWorkerCount(3)                 // Using 3 worker/process to convert
        p2i.WithTimeout(10 * time.Second)      // Must finished within 10 seconds
    )

    for _, item := task.WaitAndCollect() {
        fmt.Printf("[worker#%d] file: %s %s/%s", entry[3], entry[2], entry[0], entry[1])
    }

    // Case 4. Convert files from folder
    task, _ = p2i.ConvertFiles()
    task.Wait()

}

```

A more complex but fancy usage with mpb library, see `go-pdf2image-mpb`.


## Why and how

Converting a large PDF into images may be a time-consuming job. Thus, providing an interface to query current progress could.

## TODO

+ [ ] `outputFileFn()` to specify output filename by function.
+ [ ] `Converts()` function which support concurrently convert multiple files.
+ [ ] implement `WithSize()` option
    - `400` will fit the image to a 400x400 box, preserving aspect ratio
    - `[400, nil]` will make the image 400 pixels wide, preserving aspect ratio
    - `[500, 500]` will resize the image to 500x500 pixels, not preserving aspect ratio

## Limitations / known issues

1. Not work well with filename or path that contains CJK characters(this may caused by poppler)

## Credit

Thanks [Edouard Belval](https://github.com/Belval) for his original Python library [pdf2image](https://github.com/Belval/pdf2image) which inspires this Golang version.