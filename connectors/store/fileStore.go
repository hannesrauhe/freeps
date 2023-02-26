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

	"github.com/hannesrauhe/freeps/freepsgraph"
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

func (p *fileStoreNamespace) CompareAndSwap(key string, expected string, newValue *freepsgraph.OperatorIO, modifiedBy string) *freepsgraph.OperatorIO {
	return freepsgraph.MakeOutputError(http.StatusNotImplemented, "file support not fully implemented yet")
}

func (p *fileStoreNamespace) DeleteOlder(maxAge time.Duration) int {
	panic("not implemented") // TODO: Implement
}

func (p *fileStoreNamespace) DeleteValue(key string) {
	panic("not implemented") // TODO: Implement
}

func (p *fileStoreNamespace) GetAllValues() map[string]*freepsgraph.OperatorIO {
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

func (p *fileStoreNamespace) GetAllFiltered(keyPattern string, valuePattern string, modifiedByPattern string, minAge time.Duration, maxAge time.Duration) map[string]*freepsgraph.OperatorIO {
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
		res[d.Name()] = StoreEntry{data: freepsgraph.MakePlainOutput("File of size: %v", info.Size()), timestamp: info.ModTime()}
	}
	return res
}

func (p *fileStoreNamespace) GetValue(key string) *freepsgraph.OperatorIO {
	path, err := p.getFilePath(key)
	if err != nil {
		return freepsgraph.MakeOutputError(http.StatusBadRequest, err.Error())
	}
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return freepsgraph.MakeOutputError(500, "Failed to open file: %v", err.Error())
	}
	return freepsgraph.MakeByteOutput(b)
}

func (p *fileStoreNamespace) GetValueBeforeExpiration(key string, maxAge time.Duration) *freepsgraph.OperatorIO {
	return freepsgraph.MakeOutputError(http.StatusNotImplemented, "file support not fully implemented yet")
}

func (p *fileStoreNamespace) OverwriteValueIfOlder(key string, io *freepsgraph.OperatorIO, maxAge time.Duration, modifiedBy string) *freepsgraph.OperatorIO {
	return freepsgraph.MakeOutputError(http.StatusNotImplemented, "file support not fully implemented yet")
}

func (p *fileStoreNamespace) SetValue(key string, io *freepsgraph.OperatorIO, modifiedBy string) error {
	path, err := p.getFilePath(key)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	b, err := io.GetBytes()
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	return err
}
