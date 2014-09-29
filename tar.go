package docker

import (
	"archive/tar"
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/docker/docker/pkg/pools"
)

func createTarStream(srcPath string) (io.ReadCloser, error) {
	// handle .dockerignore
	excludes, err := parseDockerignore(srcPath)
	if err != nil {
		return nil, err
	}

	if err := validateContextDirectory(srcPath, excludes); err != nil {
		return nil, err
	}

	pipeReader, pipeWriter := io.Pipe()
	tw := tar.NewWriter(pipeWriter)

	// since the errors here don't get returned, should we log them somehow
	go func() {
		twBuf := pools.BufioWriter32KPool.Get(nil)
		defer pools.BufioWriter32KPool.Put(twBuf)

		filepath.Walk(srcPath, func(filePath string, f os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			relFilePath, err := filepath.Rel(srcPath, filePath)
			if err != nil {
				return nil
			}

			skip, err := matches(relFilePath, excludes)
			if err != nil {
				return err
			}

			if skip {
				if f.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			_ = addTarFile(filePath, relFilePath, tw, twBuf)
			return nil
		})

		_ = tw.Close()
		_ = pipeWriter.Close()
	}()

	return pipeReader, nil
}

func matches(relFilePath string, patterns []string) (bool, error) {
	for _, exclude := range patterns {
		matched, err := filepath.Match(exclude, relFilePath)
		if err != nil {
			return false, err
		}
		if matched {
			if filepath.Clean(relFilePath) == "." {
				// Can't exclude whole path
				continue
			}
			return true, nil
		}
	}
	return false, nil
}

func addTarFile(path, name string, tw *tar.Writer, twBuf *bufio.Writer) error {
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}

	link := ""
	if fi.Mode()&os.ModeSymlink != 0 {
		if link, err = os.Readlink(path); err != nil {
			return err
		}
	}

	hdr, err := tar.FileInfoHeader(fi, link)
	if err != nil {
		return err
	}

	if fi.IsDir() && !strings.HasSuffix(name, "/") {
		name = name + "/"
	}

	hdr.Name = name

	stat, ok := fi.Sys().(*syscall.Stat_t)
	if ok {
		// Currently go does not fill in the major/minors
		if stat.Mode&syscall.S_IFBLK == syscall.S_IFBLK ||
			stat.Mode&syscall.S_IFCHR == syscall.S_IFCHR {
			hdr.Devmajor = int64(major(uint64(stat.Rdev)))
			hdr.Devminor = int64(minor(uint64(stat.Rdev)))
		}
	}

	hdr = addXattrs(hdr, path)

	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}

	if hdr.Typeflag == tar.TypeReg {
		file, err := os.Open(path)
		if err != nil {
			return err
		}

		twBuf.Reset(tw)
		_, err = io.Copy(twBuf, file)
		file.Close()
		if err != nil {
			return err
		}
		err = twBuf.Flush()
		if err != nil {
			return err
		}
		twBuf.Reset(nil)
	}

	return nil
}

func major(device uint64) uint64 {
	return (device >> 8) & 0xfff
}

func minor(device uint64) uint64 {
	return (device & 0xff) | ((device >> 12) & 0xfff00)
}

// validateContextDirectory checks if all the contents of the directory
// can be read and returns an error if some files can't be read.
// Symlinks which point to non-existing files don't trigger an error
func validateContextDirectory(srcPath string, excludes []string) error {
	return filepath.Walk(filepath.Join(srcPath, "."), func(filePath string, f os.FileInfo, err error) error {
		// skip this directory/file if it's not in the path, it won't get added to the context
		if relFilePath, err := filepath.Rel(srcPath, filePath); err != nil {
			return err
		} else if skip, err := matches(relFilePath, excludes); err != nil {
			return err
		} else if skip {
			if f.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if err != nil {
			if os.IsPermission(err) {
				return fmt.Errorf("can't stat '%s'", filePath)
			}
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}

		// skip checking if symlinks point to non-existing files, such symlinks can be useful
		// also skip named pipes, because they hanging on open
		if f.Mode()&(os.ModeSymlink|os.ModeNamedPipe) != 0 {
			return nil
		}

		if !f.IsDir() {
			currentFile, err := os.Open(filePath)
			if err != nil && os.IsPermission(err) {
				return fmt.Errorf("no permission to read from '%s'", filePath)
			}
			currentFile.Close()
		}
		return nil
	})
}

func parseDockerignore(root string) ([]string, error) {
	var excludes []string
	ignore, err := ioutil.ReadFile(path.Join(root, ".dockerignore"))
	if err != nil && !os.IsNotExist(err) {
		return excludes, fmt.Errorf("error reading .dockerignore: '%s'", err)
	}
	for _, pattern := range strings.Split(string(ignore), "\n") {
		matches, err := filepath.Match(pattern, "Dockerfile")
		if err != nil {
			return excludes, fmt.Errorf("bad .dockerignore pattern: '%s', error: %s", pattern, err)
		}
		if matches {
			return excludes, fmt.Errorf("dockerfile was excluded by .dockerignore pattern '%s'", pattern)
		}
		excludes = append(excludes, pattern)
	}

	return excludes, nil
}
