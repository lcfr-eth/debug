package macho

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io/ioutil"
	"log"
	"os"
	"sort"
)

// Write - Writes an *macho.File to disk
func (machoFile *File) Write(destFile string) error {

	bytesWritten := uint64(0)
	f, err := os.Create(destFile)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)

	// Write entire file header.
	buf := &bytes.Buffer{}
	err = binary.Write(buf, machoFile.ByteOrder, machoFile.FileHeader)
	if err != nil {
		panic(err)
	}
	headerLength := len(buf.Bytes())
	binary.Write(w, machoFile.ByteOrder, machoFile.FileHeader)
	bytesWritten += uint64(headerLength)
	log.Printf("%x: Wrote file header of size: %v", bytesWritten, bytesWritten)

	// todo: Add a buffer of 4 bytes ?
	w.Write([]byte{0, 0, 0, 0})
	bytesWritten += 4

	// Write Load Commands Loop
	for _, singleLoad := range machoFile.Loads {
		buf2 := &bytes.Buffer{}
		err = binary.Write(buf2, machoFile.ByteOrder, singleLoad.Raw())
		if err != nil {
			panic(err)
		}
		LoadCmdLen := len(buf2.Bytes())
		binary.Write(w, machoFile.ByteOrder, singleLoad.Raw())
		bytesWritten += uint64(LoadCmdLen)
		log.Printf("%x: Wrote Load Command, total size of: %v", bytesWritten, LoadCmdLen)
	}
	w.Flush()

	// Write Sections
	sortedSections := machoFile.Sections[:]
	sort.Slice(sortedSections, func(a, b int) bool { return machoFile.Sections[a].Offset < machoFile.Sections[b].Offset })
	for _, s := range sortedSections {

		log.Printf("%x: section/segment name: %s %s\n", bytesWritten, s.Name, s.Seg)

		if bytesWritten > uint64(s.Offset) {
			log.Printf("Overlapping Sections in Generated macho: %+v\n", s.Name)
			continue
		}
		if bytesWritten < uint64(s.Offset) {
			pad := make([]byte, uint64(s.Offset)-bytesWritten)
			w.Write(pad)
			bytesWritten += uint64(len(pad))
			log.Printf("%x: Padding: %+d bytes\n", bytesWritten, len(pad))
		}
		section, err := ioutil.ReadAll(s.Open())
		if err != nil {
			return err
		}
		binary.Write(w, machoFile.ByteOrder, section)
		bytesWritten += uint64(len(section))

		//if len(machoFile.Insertion) > 0 && s.Size-uint64(len(section)) == uint64(len(machoFile.Insertion)) {
		//	binary.Write(w, machoFile.ByteOrder, machoFile.Insertion)
		//	bytesWritten += uint64(len(machoFile.Insertion))
		//}
		w.Flush()
	}

	//Load command 3
	//cmd LC_SEGMENT_64
	//cmdsize 72
	//segname __LINKEDIT
	//fileoff 12288

	// Write Something Here!
	// Load command 4
	// cmd LC_DYLD_INFO_ONLY

	// Write Symbols is next I think
	symtab := machoFile.Symtab
	log.Printf("Bytes written: %d", bytesWritten)
	log.Printf("Indirect symbol offset: %d", machoFile.Dysymtab.DysymtabCmd.Indirectsymoff)
	log.Printf("Locrel offset: %d", machoFile.Dysymtab.Locreloff)
	log.Printf("Symtab offset: %d", symtab.Symoff)
	log.Printf("String table offset: %d", symtab.Stroff)
	pad := make([]byte, uint64(symtab.Symoff)-bytesWritten)
	w.Write(pad)
	log.Printf("wrote pad of: %d", uint64(symtab.Symoff)-bytesWritten)
	bytesWritten += (uint64(symtab.Symoff) - bytesWritten)

	w.Write(symtab.RawSymtab)
	log.Printf("Wrote raw symtab, length of: %d", len(symtab.RawSymtab))
	bytesWritten += uint64(len(symtab.RawSymtab))
	log.Printf("Bytes written: %d", bytesWritten)

	//log.Printf("SymTab info: %+v", symtab)
	//log.Printf("padding: %d", (uint64(symtab.Symoff) - bytesWritten))

	// Write DySymTab next!
	dysymtab := machoFile.Dysymtab
	pad2 := make([]byte, uint64(dysymtab.Indirectsymoff)-bytesWritten)
	w.Write(pad2)
	log.Printf("wrote pad of: %d", pad2)
	bytesWritten += uint64(len(pad2))
	log.Printf("Bytes written: %d", bytesWritten)
	w.Write(dysymtab.RawDysymtab)
	log.Printf("Wrote raw indirect symbols, length of: %d", len(dysymtab.RawDysymtab))
	bytesWritten += uint64(len(dysymtab.RawDysymtab))
	log.Printf("Bytes written: %d", bytesWritten)

	// Write StringTab!
	pad3 := make([]byte, uint64(symtab.Stroff)-bytesWritten)
	w.Write(pad3)
	log.Printf("wrote pad of: %d", pad3)
	bytesWritten += uint64(len(pad3))
	log.Printf("Bytes written: %d", bytesWritten)
	w.Write(symtab.RawStringtab)
	log.Printf("Wrote raw stringtab, length of: %d", len(symtab.RawStringtab))
	bytesWritten += uint64(len(symtab.RawStringtab))
	log.Printf("Bytes written: %d", bytesWritten)
	w.Flush()

	// Write The Signature Block, if it exists

	//log.Printf("SigBlock Dat: %v", machoFile.SigBlock)
	if machoFile.SigBlock != nil {
		padX := make([]byte, machoFile.SigBlock.Offset-bytesWritten)
		w.Write(padX)
		log.Printf("wrote pad of: %d", padX)
		bytesWritten += uint64(len(padX))
		log.Printf("Bytes written: %d", bytesWritten)
		w.Write(machoFile.SigBlock.RawDat)
		log.Printf("Wrote raw stringtab, length of: %d", machoFile.SigBlock.Len)
		bytesWritten += uint64(machoFile.SigBlock.Len)
		log.Printf("Bytes written: %d", bytesWritten)
		w.Flush()
	}

	// Write the Signature
	// Load command 15
	// cmd LC_CODE_SIGNATURE

	// Write 0s to the end of the final segment
	if uint64(FinalSegEnd)-bytesWritten > 0 {
		pad4 := make([]byte, uint64(FinalSegEnd)-bytesWritten)
		w.Write(pad4)
		log.Printf("wrote pad of: %d", len(pad4))
		bytesWritten += uint64(len(pad4))
		log.Printf("Bytes written: %d", bytesWritten)
		w.Flush()
	}

	w.Flush()
	log.Println("All done!")
	return nil
}
