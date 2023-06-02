package freepsstore

import (
	"errors"
	"io/fs"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
)

func newFileStoreNamespace() (*fileStoreNamespace, error) {
	dir, err := utils.GetTempDir()
	if err != nil {
		return nil, err
	}
	ns := &fileStoreNamespace{dir: dir}
	return ns, nil
}

type fileStoreNamespace struct {
	dir string
}

func (p *fileStoreNamespace) getFilePath(key string) (string, error) {
	//TODO(HR): check if key contains anything that does not belong into the filepath
	if strings.Contains(key, "/") {
		return "", errors.New("Invalid key")
	}
	return path.Join(p.dir, key), nil
}

var _ StoreNamespace = &fileStoreNamespace{}

func (p *fileStoreNamespace) CompareAndSwap(key string, expected string, newValue *base.OperatorIO, modifiedBy string) *base.OperatorIO {
	return base.MakeOutputError(http.StatusNotImplemented, "file support not fully implemented yet")
}

func (p *fileStoreNamespace) DeleteOlder(maxAge time.Duration) int {
	panic("not implemented") // TODO: Implement
}

func (p *fileStoreNamespace) DeleteValue(key string) {
	path, err := p.getFilePath(key)
	if err != nil {
		return
	}
	os.Remove(path)
}

func (p *fileStoreNamespace) GetAllValues(limit int) map[string]*base.OperatorIO {
	panic("not implemented") // TODO: Implement
}

func (p *fileStoreNamespace) GetKeys() []string {
	res := []string{}
	dirEntries, err := os.ReadDir(p.dir)
	if err != nil {
		return res
	}
	for _, d := range dirEntries {
		if d.Type().IsRegular() {
			res = append(res, d.Name())
		}
	}
	return res
}

func fileMatches(de fs.DirEntry, keyPattern, valuePattern, modifiedByPattern string, minAge, maxAge time.Duration, tnow time.Time) *fs.FileInfo {
	if !de.Type().IsRegular() {
		return nil
	}
	i, err := de.Info()
	if err != nil {
		return nil
	}
	// if minAge != 0 && v.timestamp.Add(minAge).After(tnow) {
	// 	return false
	// }
	// if maxAge != math.MaxInt64 && v.timestamp.Add(maxAge).Before(tnow) {
	// 	return false
	// }
	// if keyPattern != "" && !strings.Contains(k, keyPattern) {
	// 	return false
	// }
	// if valuePattern != "" && !strings.Contains(v.data.GetString(), valuePattern) {
	// 	return false
	// }
	// if modifiedByPattern != "" && !strings.Contains(v.modifiedBy, modifiedByPattern) {
	// 	return false
	// }
	return &i
}

func (p *fileStoreNamespace) GetAllFiltered(keyPattern string, valuePattern string, modifiedByPattern string, minAge time.Duration, maxAge time.Duration) map[string]*base.OperatorIO {
	panic("not implemented") // TODO: Implement
}

func (p *fileStoreNamespace) GetSearchResultWithMetadata(keyPattern string, valuePattern string, modifiedByPattern string, minAge time.Duration, maxAge time.Duration) map[string]StoreEntry {
	res := map[string]StoreEntry{}
	dirEntries, err := os.ReadDir(p.dir)
	if err != nil {
		return res
	}
	tnow := time.Now()
	for _, d := range dirEntries {
		i := fileMatches(d, keyPattern, valuePattern, modifiedByPattern, maxAge, maxAge, tnow)
		if i == nil {
			continue
		}
		info := *i
		res[d.Name()] = StoreEntry{data: base.MakePlainOutput("File of size: %v", info.Size()), timestamp: info.ModTime()}
	}
	return res
}

func (p *fileStoreNamespace) GetValue(key string) *base.OperatorIO {
	path, err := p.getFilePath(key)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, err.Error())
	}
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return base.MakeOutputError(500, "Failed to open file: %v", err.Error())
	}
	return base.MakeByteOutput(b)
}

func (p *fileStoreNamespace) GetValueBeforeExpiration(key string, maxAge time.Duration) *base.OperatorIO {
	return base.MakeOutputError(http.StatusNotImplemented, "file support not fully implemented yet")
}

func (p *fileStoreNamespace) OverwriteValueIfOlder(key string, io *base.OperatorIO, maxAge time.Duration, modifiedBy string) *base.OperatorIO {
	return base.MakeOutputError(http.StatusNotImplemented, "file support not fully implemented yet")
}

// Len returns the number of keys in the namespace
func (p *fileStoreNamespace) Len() int {
	return len(p.GetKeys())
}

func (p *fileStoreNamespace) SetValue(key string, io *base.OperatorIO, modifiedBy string) *base.OperatorIO {
	path, err := p.getFilePath(key)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, err.Error())
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, err.Error())
	}
	b, err := io.GetBytes()
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, err.Error())
	}
	_, err = f.Write(b)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, err.Error())
	}
	return io
}

func (p *fileStoreNamespace) UpdateTransaction(key string, fn func(*base.OperatorIO) *base.OperatorIO, modifiedBy string) *base.OperatorIO {
	return base.MakeOutputError(http.StatusNotImplemented, "file support not fully implemented yet")
}
