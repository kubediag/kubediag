//////////////////////////////////////////
// archivex.go
// Jhonathan Paulo Banczek - 2014
// jpbanczek@gmail.com - jhoonb.com
//////////////////////////////////////////

package archivex

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// interface
type Archivex interface {
	Create(name string) error
	CreateWriter(name string, w io.Writer) error
	Add(name string, file io.Reader, info os.FileInfo) error
	AddAll(dir string, includeCurrentFolder bool) error
	Close() error
}

// ArchiveWriteFunc is the closure used by an archive's AddAll method to actually put a file into an archive
// Note that for directory entries, this func will be called with a nil 'file' param
type ArchiveWriteFunc func(info os.FileInfo, file io.Reader, entryName string, fullPath string) (err error)

// ZipFile implement *zip.Writer
type ZipFile struct {
	Writer *zip.Writer
	Name   string
	out    io.Writer
}

// TarFile implement *tar.Writer
type TarFile struct {
	Writer     *tar.Writer
	Name       string
	GzWriter   *gzip.Writer
	Compressed bool
	out        io.Writer
}

// Create new file zip
func (z *ZipFile) Create(name string) error {
	// check extension .zip
	if strings.HasSuffix(name, ".zip") != true {
		if strings.HasSuffix(name, ".tar.gz") == true {
			name = strings.Replace(name, ".tar.gz", ".zip", -1)
		} else {
			name = name + ".zip"
		}
	}
	z.Name = name
	file, err := os.Create(z.Name)
	if err != nil {
		return err
	}
	z.Writer = zip.NewWriter(file)
	return nil
}

// Create a new ZIP and write it to a given writer
func (z *ZipFile) CreateWriter(name string, w io.Writer) error {
	z.Writer = zip.NewWriter(w)
	z.Name = name
	return nil
}

// Add file reader in archive zip
func (z *ZipFile) Add(name string, file io.Reader, info os.FileInfo) error {
	var header *zip.FileHeader
	if info == nil {
		header = &zip.FileHeader{
			Name:   name,
			Flags:  1 << 11, // use utf8 encoding the file Name
			Method: zip.Deflate,
		}
	} else {
		var err error
		header, err = zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
	}
	zipWriter, err := z.Writer.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(zipWriter, file)
	return err
}

// AddAll adds all files from dir in archive, recursively.
// Directories receive a zero-size entry in the archive, with a trailing slash in the header name, and no compression
func (z *ZipFile) AddAll(dir string, includeCurrentFolder bool) error {
	dir = path.Clean(dir)
	return addAll(dir, dir, includeCurrentFolder, func(info os.FileInfo, file io.Reader, entryName string, fullPath string) (err error) {
		// Create a header based off of the fileinfo
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		// If it's a file, set the compression method to deflate (leave directories uncompressed)
		if !info.IsDir() {
			header.Method = zip.Deflate
		}

		// Set the header's name to what we want--it may not include the top folder
		header.Name = entryName

		// Add a trailing slash if the entry is a directory
		if info.IsDir() {
			header.Name += "/"
		}

		// Get a writer in the archive based on our header
		writer, err := z.Writer.CreateHeader(header)
		if err != nil {
			return err
		}

		// If we have a file to write (i.e., not a directory) then pipe the file into the archive writer
		if file != nil {
			if _, err := io.Copy(writer, file); err != nil {
				return err
			}
		}

		return nil
	})
}

//Close close the zip file
func (z *ZipFile) Close() error {
	err := z.Writer.Close()
	// If the out writer supports io.Closer, Close it.
	if c, ok := z.out.(io.Closer); ok {
		c.Close()
	}
	return err
}

func (t *TarFile) configureName(name string) {
	// check the filename extension

	// if it has a .gz, we'll compress it.
	t.Compressed = strings.HasSuffix(name, ".tar.gz")

	// check to see if they have the wrong extension
	if !strings.HasSuffix(name, ".tar.gz") && !strings.HasSuffix(name, ".tar") {
		// is it .zip? replace it
		if strings.HasSuffix(name, ".zip") {
			name = strings.Replace(name, ".zip", ".tar.gz", -1)
			t.Compressed = true
		} else {
			// if it's not, add .tar
			// since we'll assume it's not compressed
			name = name + ".tar"
		}
	}

	t.Name = name
}

// Create new Tar file
func (t *TarFile) Create(name string) error {
	t.configureName(name)

	file, err := os.Create(name)
	if err != nil {
		return err
	}

	if t.Compressed {
		t.GzWriter = gzip.NewWriter(file)
		t.Writer = tar.NewWriter(t.GzWriter)
	} else {
		t.Writer = tar.NewWriter(file)
	}
	t.out = file
	return nil
}

// Create a new Tar and write it to a given writer
func (t *TarFile) CreateWriter(name string, w io.Writer) error {
	t.configureName(name)

	if t.Compressed {
		t.GzWriter = gzip.NewWriter(w)
		t.Writer = tar.NewWriter(t.GzWriter)
	} else {
		t.Writer = tar.NewWriter(w)
	}
	t.out = w
	return nil
}

// Add add byte in archive tar
func (t *TarFile) Add(name string, file io.Reader, info os.FileInfo) error {
	var header *tar.Header
	if info == nil {
		// Unfortunately, we need to know the size for the tar header, making it necessary to read the full file into memory
		// TODO: FIXME: We could check if the io.Reader supports io.Seek, so we can do a size scan, and then seek to the beginning for the read
		buf := bytes.Buffer{}
		n, err := buf.ReadFrom(file)
		if err != nil {
			return err
		}
		header = &tar.Header{
			Name:    name,
			Size:    n,
			Mode:    0666,
			ModTime: time.Now(),
		}
		err = t.Writer.WriteHeader(header)
		if err != nil {
			return err
		}
		_, err = io.Copy(t.Writer, bytes.NewReader(buf.Bytes()))
		return err
	}

	var err error
	header, err = tar.FileInfoHeader(info, name)
	if err != nil {
		return err
	}
	err = t.Writer.WriteHeader(header)
	if err != nil {
		return err
	}
	n, err := io.Copy(t.Writer, file)
	if err != nil {
		return err
	}
	if n != info.Size() {
		return errors.New("unexpected amount of bytes copied")
	}
	return err
}

// AddAll adds all files from dir in archive
// Tar does not support directories
func (t *TarFile) AddAll(dir string, includeCurrentFolder bool) error {
	dir = path.Clean(dir)

	return addAll(dir, dir, includeCurrentFolder, func(info os.FileInfo, file io.Reader, entryName string, fullPath string) (err error) {
		// Create a header based off of the fileinfo
		link := ""
		if info.Mode()&os.ModeSymlink != 0 {
			link, err = os.Readlink(fullPath)
			if err != nil {
				panic(err)
			}
		}

		header, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}

		// Set the header's name to what we want--it may not include the top folder
		header.Name = entryName

		// Write the header into the tar file
		if err := t.Writer.WriteHeader(header); err != nil {
			return err
		}

		// The directory don't need copy file
		if file == nil {
			return nil
		}

		switch header.Typeflag {
		case tar.TypeLink, tar.TypeSymlink, tar.TypeChar, tar.TypeBlock, tar.TypeDir, tar.TypeFifo:
			// header only files
		default:
			// Pipe the file into the tar
			if _, err := io.Copy(t.Writer, file); err != nil {
				return err
			}
		}

		return nil
	})
}

// Close the file Tar
func (t *TarFile) Close() error {
	err := t.Writer.Close()
	if err != nil {
		return err
	}

	if t.Compressed {
		err = t.GzWriter.Close()
		if err != nil {
			return err
		}
	}

	// If the out writer supports io.Closer, Close it.
	if c, ok := t.out.(io.Closer); ok {
		c.Close()
	}
	return err
}

func getSubDir(dir string, rootDir string, includeCurrentFolder bool) (subDir string) {
	subDir = strings.Replace(dir, rootDir, "", 1)
	// Remove leading slashes, since this is intentionally a subdirectory.
	if len(subDir) > 0 && subDir[0] == os.PathSeparator {
		subDir = subDir[1:]
	}
	subDir = path.Join(strings.Split(subDir, string(os.PathSeparator))...)

	if includeCurrentFolder {
		parts := strings.Split(rootDir, string(os.PathSeparator))
		subDir = path.Join(parts[len(parts)-1], subDir)
	}
	return
}

// addAll is used to recursively go down through directories and add each file and directory to an archive, based on an ArchiveWriteFunc given to it
func addAll(dir, rootDir string, includeCurrentFolder bool, writerFunc ArchiveWriteFunc) error {
	// Get a list of all entries in the directory, as []os.FileInfo
	fileInfos, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}

	// Loop through all entries
	for _, info := range fileInfos {
		full := filepath.Join(dir, info.Name())

		// If the entry is a file, get an io.Reader for it
		var file *os.File
		var reader io.Reader
		if !info.IsDir() {
			file, err = os.Open(full)
			if err != nil {
				return err
			}
			reader = file
		}

		// Write the entry into the archive
		subDir := getSubDir(dir, rootDir, includeCurrentFolder)
		entryName := path.Join(subDir, info.Name())
		fullPath := path.Join(dir, info.Name())
		if err := writerFunc(info, reader, entryName, fullPath); err != nil {
			if file != nil {
				file.Close()
			}
			return err
		}

		if file != nil {
			if err := file.Close(); err != nil {
				return err
			}
		}

		// If the entry is a directory, recurse into it
		if info.IsDir() {
			if err := addAll(full, rootDir, includeCurrentFolder, writerFunc); err != nil {
				return err
			}
		}
	}
	return nil
}
