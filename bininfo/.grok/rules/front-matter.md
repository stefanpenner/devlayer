<!--
---
why: This file tells Grok the front-matter rule for this repo.
uses: Grok
how: Grok reads this file at session start.
Grok applies the rule to each file it creates or edits.
---
-->

# Front matter

Grok adds a front-matter comment to each text file it creates.
Grok adds the same comment when it edits a text file that lacks one.
Grok updates the comment when the facts change.

The comment has three keys.

- `why` states why the file exists.
- `uses` names the program that uses the file.
- `how` states how that program uses the file.

Apply ASD-STE100 to each comment sentence.
Do not apply ASD-STE100 to the program code.

- Use active voice.
- Put one fact in each sentence.
- Keep a description at or under 25 words.
- Keep an instruction at or under 20 words.
- Do not use a semicolon in a comment sentence.
- Do not use a phrasal verb.
- Keep one topic in each paragraph.
- Keep a paragraph at or under 6 sentences.

Use the comment marks of the file language.

- C and Rust use a block comment.
- Shell uses `#` on each line.
- Markdown uses an HTML comment.

Put the comment at the top of the file.
Read `samples/need.c` for a short example.
Read `samples/start.c` for a long example.

Skip a binary file.
Skip a generated file.
show.sh writes `samples/out`.
Cargo writes `Cargo.lock`.

The shape is below.

```
/*
---
why: One sentence.

uses: program

how: One sentence.
---
*/
```
