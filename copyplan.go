package arena

import (
	"reflect"
	"sync"
	"unsafe"
)

func memmove(dst, src unsafe.Pointer, size uintptr) {
	copy(unsafe.Slice((*byte)(dst), size), unsafe.Slice((*byte)(src), size))
}

// ---------------------------------------------------------------------------
// Copy plan types
// ---------------------------------------------------------------------------

type copyOpKind uint8

const (
	opDirect     copyOpKind = iota // value type: direct memmove
	opString                       // string: copy bytes into arena
	opPtr                          // pointer: allocate + recurse
	opSlice                        // slice: allocate backing array + recurse per element
	opArray                        // array: inline, iterate elements
	opStruct                       // inline struct: recurse with sub-plan
	opInterface                    // interface: fall back to reflect
	opDeepCopier                   // deepCopier (arena.Map/Vector): delegate
)

type copyOp struct {
	kind     copyOpKind
	offset   uintptr
	size     uintptr
	elemSize uintptr      // for opPtr, opSlice, opArray: size of element
	arrayLen int          // for opArray: number of elements
	flatElem bool         // for opSlice, opArray: element is a single flat value
	elemType reflect.Type // for opPtr, opSlice, opArray, opInterface
	subPlan  *copyPlan    // for opPtr, opSlice, opArray, opStruct: plan for elements/fields
}

type copyPlan struct {
	ops    []copyOp
	cyclic bool
}

var copyPlanCache sync.Map // map[reflect.Type]*copyPlan

// ---------------------------------------------------------------------------
// Plan builder
// ---------------------------------------------------------------------------

func getOrBuildCopyPlan(t reflect.Type) *copyPlan {
	if v, ok := copyPlanCache.Load(t); ok {
		return v.(*copyPlan)
	}
	plan := buildCopyPlan(t, make(map[reflect.Type]*copyPlan, 32))
	plan.ops = mergeDirectOps(plan.ops)
	copyPlanCache.Store(t, plan)
	return plan
}

func buildCopyPlan(t reflect.Type, seen map[reflect.Type]*copyPlan) *copyPlan {
	if p, ok := seen[t]; ok {
		return p
	}
	if v, ok := copyPlanCache.Load(t); ok {
		return v.(*copyPlan)
	}

	info := computeTypeInfo(t, make(map[reflect.Type]bool))
	plan := &copyPlan{cyclic: info.cyclic}
	seen[t] = plan

	switch t.Kind() {
	case reflect.Struct:
		if isArenaContainerType(t) {
			// Arena containers are handled by opDeepCopier, no field ops needed
			break
		}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			plan.ops = append(plan.ops, buildFieldOp(f.Offset, f.Type, seen))
		}
	case reflect.Ptr:
		sub := buildCopyPlan(t.Elem(), seen)
		plan.ops = append(plan.ops, copyOp{
			kind:     opPtr,
			size:     t.Elem().Size(),
			elemSize: t.Elem().Size(),
			elemType: t.Elem(),
			subPlan:  sub,
		})
	case reflect.String:
		plan.ops = append(plan.ops, copyOp{kind: opString})
	case reflect.Slice:
		sub := buildCopyPlan(t.Elem(), seen)
		plan.ops = append(plan.ops, copyOp{
			kind:     opSlice,
			elemSize: t.Elem().Size(),
			elemType: t.Elem(),
			subPlan:  sub,
		})
	default:
		// Primitive/value types: single direct copy of the whole value
		plan.ops = append(plan.ops, copyOp{kind: opDirect, size: t.Size()})
	}

	return plan
}

func buildFieldOp(offset uintptr, t reflect.Type, seen map[reflect.Type]*copyPlan) copyOp {
	// Check if type implements deepCopier (arena.Map, arena.Vector)
	deepCopierType := reflect.TypeFor[deepCopier]()
	if t.Implements(deepCopierType) || reflect.PointerTo(t).Implements(deepCopierType) {
		return copyOp{kind: opDeepCopier, offset: offset, elemType: t}
	}

	switch t.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Uintptr, reflect.Float32, reflect.Float64:
		return copyOp{kind: opDirect, offset: offset, size: t.Size()}

	case reflect.String:
		return copyOp{kind: opString, offset: offset}

	case reflect.Ptr:
		sub := buildCopyPlan(t.Elem(), seen)
		return copyOp{
			kind:     opPtr,
			offset:   offset,
			size:     t.Elem().Size(),
			elemSize: t.Elem().Size(),
			elemType: t.Elem(),
			subPlan:  sub,
		}

	case reflect.Slice:
		sub := buildCopyPlan(t.Elem(), seen)
		return copyOp{
			kind:     opSlice,
			offset:   offset,
			elemSize: t.Elem().Size(),
			elemType: t.Elem(),
			subPlan:  sub,
			flatElem: len(sub.ops) == 1 && sub.ops[0].kind == opDirect && sub.ops[0].size == t.Elem().Size(),
		}

	case reflect.Array:
		sub := buildCopyPlan(t.Elem(), seen)
		return copyOp{
			kind:     opArray,
			offset:   offset,
			size:     t.Size(),
			elemSize: t.Elem().Size(),
			arrayLen: t.Len(),
			elemType: t.Elem(),
			subPlan:  sub,
			flatElem: len(sub.ops) == 1 && sub.ops[0].kind == opDirect && sub.ops[0].size == t.Elem().Size(),
		}

	case reflect.Struct:
		sub := buildCopyPlan(t, seen)
		return copyOp{kind: opStruct, offset: offset, subPlan: sub}

	case reflect.Interface:
		return copyOp{kind: opInterface, offset: offset, elemType: t}

	default:
		return copyOp{kind: opDirect, offset: offset, size: t.Size()}
	}
}

// ---------------------------------------------------------------------------
// Plan executor
// ---------------------------------------------------------------------------

func executeCopyPlan(ar *Arena, plan *copyPlan, src, dst unsafe.Pointer, visited map[uintptr]unsafe.Pointer) {
	for i := range plan.ops {
		executeOp(ar, &plan.ops[i], src, dst, visited)
	}
}

func executeOp(ar *Arena, op *copyOp, baseSrc, baseDst unsafe.Pointer, visited map[uintptr]unsafe.Pointer) {
	s := add(baseSrc, op.offset)
	d := add(baseDst, op.offset)

	switch op.kind {
	case opDirect:
		memmove(d, s, op.size)

	case opString:
		sp := *(*string)(s)
		if len(sp) == 0 {
			*(*string)(d) = ""
			return
		}
		newPtr := ar.Malloc(uintptr(len(sp)))
		memmove(newPtr, unsafe.Pointer(unsafe.StringData(sp)), uintptr(len(sp)))
		*(*string)(d) = unsafe.String((*byte)(newPtr), len(sp))

	case opPtr:
		ptr := *(*unsafe.Pointer)(s)
		if ptr == nil {
			*(*unsafe.Pointer)(d) = nil
			return
		}
		if visited != nil {
			if existing, ok := visited[uintptr(ptr)]; ok {
				*(*unsafe.Pointer)(d) = existing
				return
			}
		}
		newPtr := ar.Malloc(op.elemSize)
		*(*unsafe.Pointer)(d) = newPtr
		if visited != nil {
			visited[uintptr(ptr)] = newPtr
		}
		if op.subPlan != nil {
			executeCopyPlan(ar, op.subPlan, ptr, newPtr, visited)
		}

	case opSlice:
		// Read slice header: {Data uintptr, Len int, Cap int}
		sliceSrc := (*reflect.SliceHeader)(s)
		if sliceSrc.Data == 0 {
			// nil slice
			*(*unsafe.Pointer)(d) = nil
			return
		}
		length := int(sliceSrc.Len)
		capacity := int(sliceSrc.Cap)
		elemSize := op.elemSize

		backingPtr := ar.Malloc(uintptr(capacity) * elemSize)
		sliceDst := (*reflect.SliceHeader)(d)
		sliceDst.Data = uintptr(backingPtr)
		sliceDst.Len = length
		sliceDst.Cap = capacity

		if op.flatElem {
			memmove(backingPtr, unsafe.Pointer(sliceSrc.Data), uintptr(length)*elemSize)
		} else if op.subPlan != nil && len(op.subPlan.ops) > 0 {
			for i := 0; i < length; i++ {
				elemSrc := add(unsafe.Pointer(sliceSrc.Data), uintptr(i)*elemSize)
				elemDst := add(backingPtr, uintptr(i)*elemSize)
				executeCopyPlan(ar, op.subPlan, elemSrc, elemDst, visited)
			}
		} else {
			memmove(backingPtr, unsafe.Pointer(sliceSrc.Data), uintptr(length)*elemSize)
		}

	case opArray:
		// Arrays are inline in the struct — no header, just contiguous elements
		if op.flatElem {
			memmove(d, s, op.size)
		} else if op.subPlan != nil && len(op.subPlan.ops) > 0 {
			for i := 0; i < op.arrayLen; i++ {
				elemSrc := add(s, uintptr(i)*op.elemSize)
				elemDst := add(d, uintptr(i)*op.elemSize)
				executeCopyPlan(ar, op.subPlan, elemSrc, elemDst, visited)
			}
		} else {
			memmove(d, s, op.size)
		}

	case opStruct:
		if op.subPlan != nil {
			executeCopyPlan(ar, op.subPlan, s, d, visited)
		}

	case opInterface:
		// Fall back to reflect for interface fields
		srcVal := reflect.NewAt(op.elemType, s).Elem()
		if srcVal.IsNil() {
			return
		}
		concrete := srcVal.Elem()
		concreteType := concrete.Type()
		newPtr := ar.Malloc(concreteType.Size())
		newConcrete := reflect.NewAt(concreteType, newPtr).Elem()
		deepCopy(ar, concrete, newConcrete, visited)
		dstVal := reflect.NewAt(op.elemType, d).Elem()
		dstVal.Set(newConcrete.Addr())

	case opDeepCopier:
		srcVal := reflect.NewAt(op.elemType, s).Elem()
		if dc, ok := srcVal.Interface().(deepCopier); ok {
			result := dc.arenaDeepCopy(ar)
			dstVal := reflect.NewAt(op.elemType, d).Elem()
			dstVal.Set(result)
		}
	}
}

func add(ptr unsafe.Pointer, offset uintptr) unsafe.Pointer {
	return unsafe.Pointer(uintptr(ptr) + offset)
}

func mergeDirectOps(ops []copyOp) []copyOp {
	if len(ops) <= 1 {
		return ops
	}
	merged := make([]copyOp, 0, len(ops))
	i := 0
	for i < len(ops) {
		if ops[i].kind != opDirect {
			merged = append(merged, ops[i])
			i++
			continue
		}
		// Start of a potential merge run
		start := ops[i]
		j := i + 1
		for j < len(ops) && ops[j].kind == opDirect && start.offset+start.size == ops[j].offset {
			start.size += ops[j].size
			j++
		}
		merged = append(merged, start)
		i = j
	}
	return merged
}
