package mfile

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

type Ts struct {
	once sync.Once
}

var ts *Ts = new(Ts)

func New() *Ts {
	t := ts
	t.once.Do(func() {
	})
	return t
}

func (t *Ts) CreateEmptyFile(path string) error {
	return CreateEmptyFile(path)
}

func (t *Ts) CreateEmptyDir(path string) error {
	return CreateEmptyDir(path)
}

func (t *Ts) CreateDirPath(filePath string) error {
	return CreateDirPath(filePath)
}

func (t *Ts) IsEmptyDir(path string) (bool, error) {
	return IsEmptyDir(path)
}

func (t *Ts) ExtractLines(buffer *bytes.Buffer, numLines int, skipBlankLines bool, existingLines []string) ([]string, *bytes.Buffer, error) {
	return ExtractLines(buffer, numLines, skipBlankLines, existingLines)
}

func (t *Ts) TailN(filePath string, numLines int, skipBlankLines bool) ([]string, error) {
	return TailN(filePath, numLines, skipBlankLines)
}

func CreateEmptyFile(path string) error {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			// 创建文件所在的目录
			os.MkdirAll(filepath.Dir(path), os.ModePerm)
			// 创建文件
			_, err = os.Create(path)
			if err != nil {
				// 处理创建文件时可能发生的错误
				return err
			}
		}
	}
	return nil
}

func CreateEmptyDir(path string) error {
	err := os.MkdirAll(path, os.ModePerm)
	if err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}
	return nil
}

func CreateDirPath(filePath string) error {
	dir := filepath.Dir(filePath)
	// 创建目录（如果不存在）
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}
	return nil
}

func IsEmptyDir(path string) (bool, error) {
	// 获取路径的信息
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	// 确认路径是一个目录
	if !info.IsDir() {
		return false, fmt.Errorf("路径 %s 不是一个目录", path)
	}

	// 打开目录
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	// 尝试读取一个条目
	_, err = f.Readdir(1)
	if err == io.EOF {
		// 目录为空
		return true, nil
	}
	if err != nil {
		return false, err
	}

	// 目录不为空
	return false, nil
}

func ExtractLines(buffer *bytes.Buffer, numLines int, skipBlankLines bool, existingLines []string) ([]string, *bytes.Buffer, error) {
	data := buffer.Bytes()
	var newLines []string

	// 从缓冲区末尾开始查找换行符
	for i := len(data) - 1; i >= 0; i-- {
		if data[i] == '\n' {
			// 提取从 i+1 到末尾的内容作为一行
			line := string(data[i+1:])
			if skipBlankLines && len(line) == 0 {
				// 跳过空行
			} else {
				newLines = append(newLines, line)
				if len(existingLines)+len(newLines) >= numLines {
					break
				}
			}
			// 更新缓冲区为已处理部分
			data = data[:i]
		}
	}

	// 反转 newLines，因为是从文件末尾向前读取的
	for j, k := 0, len(newLines)-1; j < k; j, k = j+1, k-1 {
		newLines[j], newLines[k] = newLines[k], newLines[j]
	}

	// 添加到现有行中
	existingLines = append(newLines, existingLines...)

	// 更新缓冲区为未处理的部分
	newBuffer := &bytes.Buffer{}
	newBuffer.Write(data)
	buffer = newBuffer

	return existingLines, buffer, nil
}

// TailN 从文件末尾读取最后 numLines 行，每次读取128字节
func TailN(filePath string, numLines int, skipBlankLines bool) ([]string, error) {
	var ret []string // 定义返回切片

	if numLines <= 0 {
		return ret, errors.New("numLines must be greater than 0")
	}

	// 打开文件
	file, err := os.Open(filePath)
	if err != nil {
		return ret, err // 出现错误时返回空切片和错误
	}
	defer file.Close()

	// 获取文件信息
	fileInfo, err := file.Stat()
	if err != nil {
		return ret, err // 出现错误时返回空切片和错误
	}
	fileSize := fileInfo.Size()

	var (
		buffer    = &bytes.Buffer{} // 定义为指针类型
		lines     []string
		remaining       = fileSize
		chunkSize int64 = 122 // 每次读取128字节
	)

	for remaining > 0 && len(lines) < numLines {
		if remaining < chunkSize {
			chunkSize = remaining
		}
		readOffset := remaining - chunkSize

		// 设置读取位置
		_, err := file.Seek(readOffset, 0)
		if err != nil {
			return ret, err // 出现错误时返回空切片和错误
		}

		// 读取块
		chunk := make([]byte, chunkSize)
		n, err := file.Read(chunk)
		if err != nil {
			return ret, err // 出现错误时返回空切片和错误
		}
		if int64(n) != chunkSize {
			return ret, errors.New("failed to read the expected number of bytes")
		}

		// 将当前块添加到缓冲区的前面
		// 创建一个新的缓冲区，将新的块写入，再写入现有的缓冲区内容
		newBuffer := &bytes.Buffer{}
		newBuffer.Write(chunk)
		newBuffer.Write(buffer.Bytes())
		buffer = newBuffer

		remaining = readOffset

		// 从缓冲区中提取行
		lines, buffer, err = ExtractLines(buffer, numLines, skipBlankLines, lines)
		if err != nil {
			return ret, err // 出现错误时返回空切片和错误
		}
		if len(lines) >= numLines {
			break
		}
	}

	// 处理文件开头可能缺少的换行符
	if remaining == 0 && buffer.Len() > 0 && len(lines) < numLines {
		line := buffer.String()
		if !(skipBlankLines && len(line) == 0) {
			lines = append([]string{line}, lines...)
		}
	}

	// 确保返回的行数不超过 numLines
	if len(lines) > numLines {
		lines = lines[len(lines)-numLines:]
	}

	ret = lines
	return ret, nil
}
