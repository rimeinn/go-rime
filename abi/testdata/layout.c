#include <stddef.h>
#include <stdio.h>
#include <rime_api.h>
#include <rime_levers_api.h>

#define LAYOUT(Type) printf(#Type " %zu %zu\n", sizeof(Type), (size_t)_Alignof(Type));
#define FIELD(Type, member) printf(#Type "." #member " %zu\n", offsetof(Type, member));
#define INIT(Type) do { RIME_STRUCT(Type, value); printf(#Type ".init %d\n", value.data_size); } while (0);

int main(void) {
  LAYOUT(Bool)
  LAYOUT(RimeSessionId)

  LAYOUT(RimeTraits)
  INIT(RimeTraits)
  FIELD(RimeTraits, data_size)
  FIELD(RimeTraits, shared_data_dir)
  FIELD(RimeTraits, user_data_dir)
  FIELD(RimeTraits, distribution_name)
  FIELD(RimeTraits, distribution_code_name)
  FIELD(RimeTraits, distribution_version)
  FIELD(RimeTraits, app_name)
  FIELD(RimeTraits, modules)
  FIELD(RimeTraits, min_log_level)
  FIELD(RimeTraits, log_dir)
  FIELD(RimeTraits, prebuilt_data_dir)
  FIELD(RimeTraits, staging_dir)

  LAYOUT(RimeComposition)
  FIELD(RimeComposition, length)
  FIELD(RimeComposition, cursor_pos)
  FIELD(RimeComposition, sel_start)
  FIELD(RimeComposition, sel_end)
  FIELD(RimeComposition, preedit)

  LAYOUT(RimeCandidate)
  FIELD(RimeCandidate, text)
  FIELD(RimeCandidate, comment)
  FIELD(RimeCandidate, reserved)

  LAYOUT(RimeMenu)
  FIELD(RimeMenu, page_size)
  FIELD(RimeMenu, page_no)
  FIELD(RimeMenu, is_last_page)
  FIELD(RimeMenu, highlighted_candidate_index)
  FIELD(RimeMenu, num_candidates)
  FIELD(RimeMenu, candidates)
  FIELD(RimeMenu, select_keys)

  LAYOUT(RimeCommit)
  INIT(RimeCommit)
  FIELD(RimeCommit, data_size)
  FIELD(RimeCommit, text)

  LAYOUT(RimeContext)
  INIT(RimeContext)
  FIELD(RimeContext, data_size)
  FIELD(RimeContext, composition)
  FIELD(RimeContext, menu)
  FIELD(RimeContext, commit_text_preview)
  FIELD(RimeContext, select_labels)

  LAYOUT(RimeStatus)
  INIT(RimeStatus)
  FIELD(RimeStatus, data_size)
  FIELD(RimeStatus, schema_id)
  FIELD(RimeStatus, schema_name)
  FIELD(RimeStatus, is_disabled)
  FIELD(RimeStatus, is_composing)
  FIELD(RimeStatus, is_ascii_mode)
  FIELD(RimeStatus, is_full_shape)
  FIELD(RimeStatus, is_simplified)
  FIELD(RimeStatus, is_traditional)
  FIELD(RimeStatus, is_ascii_punct)

#ifdef HAVE_RIME_CANDIDATE_PREVIEW
  LAYOUT(RimeCandidatePreview)
  INIT(RimeCandidatePreview)
  FIELD(RimeCandidatePreview, data_size)
  FIELD(RimeCandidatePreview, text_before_selection)
  FIELD(RimeCandidatePreview, selected_text)
  FIELD(RimeCandidatePreview, text_after_selection)
#endif

  LAYOUT(RimeCandidateListIterator)
  FIELD(RimeCandidateListIterator, ptr)
  FIELD(RimeCandidateListIterator, index)
  FIELD(RimeCandidateListIterator, candidate)

  LAYOUT(RimeConfig)
  FIELD(RimeConfig, ptr)

  LAYOUT(RimeConfigIterator)
  FIELD(RimeConfigIterator, list)
  FIELD(RimeConfigIterator, map)
  FIELD(RimeConfigIterator, index)
  FIELD(RimeConfigIterator, key)
  FIELD(RimeConfigIterator, path)

  LAYOUT(RimeSchemaList)
  FIELD(RimeSchemaList, size)
  FIELD(RimeSchemaList, list)

  LAYOUT(RimeSchemaListItem)
  FIELD(RimeSchemaListItem, schema_id)
  FIELD(RimeSchemaListItem, name)
  FIELD(RimeSchemaListItem, reserved)

  LAYOUT(RimeStringSlice)
  FIELD(RimeStringSlice, str)
  FIELD(RimeStringSlice, length)

  LAYOUT(RimeModule)
  INIT(RimeModule)
  FIELD(RimeModule, data_size)
  FIELD(RimeModule, module_name)
  FIELD(RimeModule, initialize)
  FIELD(RimeModule, finalize)
  FIELD(RimeModule, get_api)

  LAYOUT(RimeCustomApi)
  INIT(RimeCustomApi)
  FIELD(RimeCustomApi, data_size)

  LAYOUT(RimeUserDictIterator)
  FIELD(RimeUserDictIterator, ptr)
  FIELD(RimeUserDictIterator, i)

  LAYOUT(RimeApi)
  INIT(RimeApi)
  FIELD(RimeApi, data_size)
  LAYOUT(RimeLeversApi)
  INIT(RimeLeversApi)
  FIELD(RimeLeversApi, data_size)
#include "layout_slots.h"

  return 0;
}
