# Task 3 — AI blind test generation

> Homework — Task 3 (Lesson 6: File I/O, JSON and Testing).
> This task is **not auto-graded** by CI — it's reviewed by your mentor.
> The workflow only checks that you actually filled this file in
> (see the "Task 3 — AI report present" step in the Actions log).

## Instructions

1. Pick one function you wrote for Task 1 or Task 2 (e.g. `LoadTodos` or
   `ValidateEmail`).
2. Give an AI assistant **only the function signature** — no explanation
   of the logic, no existing test file. Ask it to generate a full
   table-driven test suite for that signature.
3. Run the AI-generated tests against your implementation.
4. Fill in the sections below.

---

## Function under test

`func ValidateEmail(s string) bool` from validate/validate.go

## Prompt you gave the AI

Please without reading repo files - func ValidateEmail(s string) bool  generate a full table-driven test suite for that signature. Dont
read any of repo files, that function is in validate.go but dont read it, create blind tests in validate_ai_test.go

## Edge cases the AI found that you had missed

None, only two FP , commented them out 

## Edge cases you had that the AI missed

None 

## Cases where the AI's expected output was wrong

valid/all_safe_symbols
valid/local_percent

## What you'd change about your own test-writing process after this

Way overcomplicated for testing a  ValidateEmail , LLM was quite focused on trying to break it, so it did everything it could on this specific task, therefore - overcommited in my opinion. 