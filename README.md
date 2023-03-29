# pico - convert PDF to images with progress

A Go implementation for [@Belval](https://github.com/Belval)'s Python [pdf2image](https://github.com/Belval/pdf2image) but with progress support.

## Install and dependency

```
$ go get github.com/DeathKing/pico/...
```

`poppler` installation manual is copied from [@Belval/pdf2image](https://github.com/Belval/pdf2image#how-to-install).

### Windows

Windows users will have to build or download poppler for Windows. I recommend [@oschwartz10612 version](https://github.com/oschwartz10612/poppler-windows/releases/) which is the most up-to-date. You will then have to add the `bin/` folder to [PATH](https://www.architectryan.com/2018/03/17/add-to-the-path-on-windows-10/) or use `WithPopperPath("C:/path/to/poppler-xx/bin")`.

### Mac

Mac users will have to install [poppler](https://poppler.freedesktop.org/).

Installing using [Brew](https://brew.sh/):

```
brew install poppler
```

### Linux

Most distros ship with `pdftoppm` and `pdftocairo`. If they are not installed, refer to your package manager to install `poppler-utils`

### Platform-independant (Using `conda`)

1. Install poppler: `conda install -c conda-forge poppler`
2. Install pdf2image: `pip install pdf2image`

## Usage

### Programmatically use it as a library

```go
import "github.com/DeathKing/pico"

func main() {

    // Case 1. Silently convert file with single worker
    task, _ := pico.Convert("path/to/pdf")
    task.Wait()

    // Case 2. Convert file with multiple worker, instead of `Wait()` for final
    //         result, we take the per-page conversion result through `Entries`
    //         channel
    task, _ = pico.Convert("path/to/pdf",
        pico.WithJob(4),
    )

    // entry ["current_page" "total_page" "output_filename"]
    for entry := range task.Entries {
        converted, totoal := t.Progress()
        fmt.Printf("[%d/%d] page %s is converted` as file %s \n",
            converted,
            total,
            entry[0], // current page
            entry[2], // output filename
        )
    }

    // Case 3. A more fancy usage
    task, _ = pico.Convert("path/to/pdf",
        pico.WithPopperPath("path/to/poppler"),
        pico.WithFormat("jpg"),
        pico.WithDPI(72),
        pico.WithPageRange(22, 42),             // Convert from Page 22 to Page 42 (included)
        pico.WithJob(3)                         // Using 3 worker/process to convert
        pico.WithTimeout(10 * time.Second)      // Must finished within 10 seconds
    )

    for _, item := task.WaitAndCollect() {
        fmt.Printf("[worker#%d] file: %s %s/%s", entry[3], entry[2], entry[0], entry[1])
    }

    // Case 4. Convert files from folder
    task, _ = pico.ConvertFiles()
    task.Wait()
}

```

### Use it as a command line tool

A more complex but fancy usage with mpb library, see `cmd/main.go`.


## Why and how

Converting a large PDF into images may be a time-consuming job. Thus, providing an interface to query current progress could.

## TODO

+ [x] `outputFileFn()` to specify output filename by function.
+ [x] `Converts()` function which support concurrently convert multiple files.
+ [x] implement `WithScale()/WithSize()/WithScaleToX()/WithScaleToY` option
    - `WithScale(400)` or `WithSize(400)` will fit the image to a 400x400 box, preserving aspect ratio
    - `WithScaleToX(400)` will make the image 400 pixels wide, preserving aspect ratio
    - `WithScaleToY(400)` will make the image 400 pixels height, preserving aspect ratio
+ [ ] more test cases.

## Limitations / known issues

1. Not work well with filename or path that contains CJK characters(this may caused by poppler)

## Credit

Thanks [Edouard Belval](https://github.com/Belval) for his original Python library [pdf2image](https://github.com/Belval/pdf2image) which inspires this Golang version.