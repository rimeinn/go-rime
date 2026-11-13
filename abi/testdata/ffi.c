#include <stddef.h>
#include <stdint.h>

#if defined(_WIN32)
#define EXPORT __declspec(dllexport)
#else
#define EXPORT __attribute__((visibility("default")))
#endif

#ifndef NO_ENTRY_POINT
typedef void (*handler)(void *, uintptr_t, const char *, const char *);

typedef struct {
  const char *str;
  size_t length;
} string_slice;

typedef struct {
  int data_size;
  void (*setup)(void *);
  void (*set_notification_handler)(handler, void *);
  void (*initialize)(void *);
  void (*finalize)(void);
  int (*start_maintenance)(int);
} short_api;

static void notify(handler callback, void *context) {
  if (callback) callback(context, UINT64_C(0x100000005), "type", "测试");
}

static int maintenance(int full_check) {
  return full_check == -123 ? -7 : 0;
}

#ifndef STATE_LABEL_SLOT
#define STATE_LABEL_SLOT 88
#endif

/* All intervening fields have the size and alignment of function pointers. */
typedef struct {
  short_api head;
  void (*unused[STATE_LABEL_SLOT - 5])(void);
  string_slice (*get_state_label_abbreviated)(uintptr_t, const char *, int, int);
} full_api;

static string_slice state_label(uintptr_t session, const char *option, int state, int abbreviated) {
  static const char bytes[] = {'a', '\0', 'b', 'c'};
  string_slice out = {0};
  if (session == UINT64_C(0x100000005) && option[0] == 'x' && state == 1 && abbreviated == -12) {
    out.str = bytes;
    out.length = 3;
  }
  return out;
}

EXPORT void *rime_get_api(void) {
#if defined(NULL_TABLE)
  return 0;
#elif defined(NEGATIVE_TABLE)
  static const int size = -1;
  return (void *)&size;
#elif defined(FULL_TABLE)
  static full_api api = {
    {sizeof(full_api) - sizeof(int), 0, notify, 0, 0, maintenance},
    {0}, state_label
  };
  return &api;
#else
  /* Advertise one byte less than the fifth pointer to exercise a partial slot. */
  static short_api api = {sizeof(short_api) - sizeof(int) - 1, 0, notify, 0, 0, maintenance};
  return &api;
#endif
}
#else
EXPORT void unrelated_symbol(void) {}
#endif
