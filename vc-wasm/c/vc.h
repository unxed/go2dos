/* vc.h — среда перевода Volkov Commander 4.05 в C (соглашения — vc-wasm/PLAN.md, §3).
 *
 * Перевод механический: каждая команда ассемблера — одна-две строки C рядом с
 * исходной командой в комментарии. Состояние x86 (регистры, флаги, память) —
 * глобальное, как у настоящего процессора; процедура VC — функция без
 * параметров.
 */
#ifndef VC_H
#define VC_H

#include <stdint.h>

/* Регистры в порядке кодирования x86 (как cpu.AX..cpu.DI в go2dos). Раскладка
 * структуры — часть ABI с хостом и тестами: не менять. */
typedef struct {
    uint16_t ax, cx, dx, bx, sp, bp, si, di; /* смещения 0..14  */
    uint16_t es, cs, ss, ds;                 /* смещения 16..22 */
    uint16_t flags;                          /* смещение 24     */
} Regs;

extern Regs r;

/* Адресное пространство реального режима: линейный адрес seg*16+off (с
 * переносом по модулю 1 МиБ, как у 8086). Арена — его начало; размер — при
 * сборке. Обращение за пределы арены — trap (fail fast). */
#ifndef ARENA_SIZE
#define ARENA_SIZE 0x100000u
#endif
extern uint8_t arena[ARENA_SIZE];

/* VC_SMALL: доступ к памяти — вызовы, а не встраивание (меньше кода, медленнее).
 * Это ручка размера: переведённый код от неё не зависит (PLAN §3, П10). */
#ifdef VC_SMALL
#define VC_ACC static __attribute__((noinline))
#else
#define VC_ACC static inline
#endif

static inline uint32_t lin(uint16_t seg, uint16_t off) {
    uint32_t a = (((uint32_t)seg << 4) + off) & 0xFFFFFu;
#if ARENA_SIZE < 0x100000u
    if (a >= ARENA_SIZE) __builtin_trap();
#endif
    return a;
}
VC_ACC uint8_t rb(uint16_t s, uint16_t o) { return arena[lin(s, o)]; }
VC_ACC void wb(uint16_t s, uint16_t o, uint8_t v) { arena[lin(s, o)] = v; }
VC_ACC uint16_t rw(uint16_t s, uint16_t o) {
    uint32_t a = lin(s, o);
    if (o != 0xFFFFu && a + 1 < ARENA_SIZE) { /* обычный случай: одно чтение слова */
        uint16_t v;
        __builtin_memcpy(&v, &arena[a], 2);
        return v;
    }
    return (uint16_t)(arena[a] | rb(s, (uint16_t)(o + 1)) << 8); /* слово через 0FFFFh — как у 8086 */
}
VC_ACC void ww(uint16_t s, uint16_t o, uint16_t v) {
    uint32_t a = lin(s, o);
    if (o != 0xFFFFu && a + 1 < ARENA_SIZE) {
        __builtin_memcpy(&arena[a], &v, 2);
        return;
    }
    arena[a] = (uint8_t)v;
    wb(s, (uint16_t)(o + 1), (uint8_t)(v >> 8));
}

/* Половинки регистров. */
#define AL() ((uint8_t)r.ax)
#define AH() ((uint8_t)(r.ax >> 8))
#define BL() ((uint8_t)r.bx)
#define BH() ((uint8_t)(r.bx >> 8))
#define CL() ((uint8_t)r.cx)
#define CH() ((uint8_t)(r.cx >> 8))
#define DL() ((uint8_t)r.dx)
#define DH() ((uint8_t)(r.dx >> 8))
#define SET_LO(R, v) ((R) = (uint16_t)(((R) & 0xFF00u) | (uint8_t)(v)))
#define SET_HI(R, v) ((R) = (uint16_t)(((R) & 0x00FFu) | (uint16_t)(uint8_t)(v) << 8))
#define SET_AL(v) SET_LO(r.ax, v)
#define SET_AH(v) SET_HI(r.ax, v)
#define SET_BL(v) SET_LO(r.bx, v)
#define SET_BH(v) SET_HI(r.bx, v)
#define SET_CL(v) SET_LO(r.cx, v)
#define SET_CH(v) SET_HI(r.cx, v)
#define SET_DL(v) SET_LO(r.dx, v)
#define SET_DH(v) SET_HI(r.dx, v)

/* Флаги (биты FLAGS). Вычисляются только те, что кто-то читает (PLAN §3, П4). */
enum { CF = 0x0001, PF = 0x0004, AF = 0x0010, ZF = 0x0040, SF = 0x0080,
       DF = 0x0400, OF = 0x0800 };
#define GETF(f) ((r.flags & (f)) != 0)
#define SETF(f, on) (r.flags = (on) ? (uint16_t)(r.flags | (f)) : (uint16_t)(r.flags & ~(f)))
#define CLD() SETF(DF, 0)
#define STD() SETF(DF, 1)

/* Стек — настоящий, в арене по SS:SP: раскладка байт совпадает с оригиналом. */
static inline void PUSH(uint16_t v) { r.sp -= 2; ww(r.ss, r.sp, v); }
static inline uint16_t POP(void) { uint16_t v = rw(r.ss, r.sp); r.sp += 2; return v; }

/* Строковые команды учитывают DF (PLAN §3, П7). Источник — DS (или префикс),
 * приёмник — всегда ES. Недостающие (MOVS, CMPS, SCAS, REP) добавлять сюда. */
#define DSTEP(n) (GETF(DF) ? (uint16_t)(0x10000u - (n)) : (uint16_t)(n))
static inline void STOSB(void) { wb(r.es, r.di, AL()); r.di += DSTEP(1); }
static inline void STOSW(void) { ww(r.es, r.di, r.ax); r.di += DSTEP(2); }
static inline void LODSB(uint16_t seg) { SET_AL(rb(seg, r.si)); r.si += DSTEP(1); }
static inline void LODSW(uint16_t seg) { r.ax = rw(seg, r.si); r.si += DSTEP(2); }

/* Вызовы (PLAN §3, П5). CALL кладёт в стек настоящий адрес возврата из листинга,
 * RET его снимает; переход JMP в другую процедуру — TAILJMP, без PUSH. */
#define CALL(proc, retaddr) do { PUSH(retaddr); proc(); } while (0)
#define RET() do { r.sp += 2; return; } while (0)
#define RETN(n) do { r.sp += 2 + (n); return; } while (0)
#define TAILJMP(proc) do { proc(); return; } while (0)
#define CALLF(proc, retseg, retoff) do { PUSH(retseg); PUSH(retoff); proc(); } while (0)
#define RETF() do { r.sp += 4; return; } while (0)

/* Сбой перевода: недопустимое место (не переведено, неизвестный адрес). */
#define TRAP() __builtin_trap()

#define EXPORT(name) __attribute__((export_name(#name)))

#endif
