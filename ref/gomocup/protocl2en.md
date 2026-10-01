source: https://plastovicka.github.io/protocl2en.htm

# Gomoku AI protocol


[Back to information](https://gomocup.org/detail-information/)

## Introduction

Each line contains exactly one command (there is only one exception). The manager puts bytes CR LF (0x0d, 0x0a) at the end of lines. The brain can send lines that are ended with CR LF, or just LF, or CR. The manager ignores lines that are empty. It must not crash if a line is too long, but it can silently cut very long lines.

If the brain has only one thread, it is very important not to read from input when the brain is required to think or respond to a command. It would lead to deadlock (both brain and manager are waiting). Manager will terminate the brain in this situation after the time for a turn is out. The brain can have two threads to avoid that problem. The first thread reads commands from input and the second thread thinks and writes responds to output. It is necessary to use some synchronization objects (events, locks or semaphores). The number of threads is not important for a tournament. One thread is sufficient for a tournament. But two threads are useful for human players. For example, someone may want to change time limits while the brain already started to think. The thinking can also be easily canceled at any time without terminating the brain.

The brain is required to process commands START, BEGIN, INFO, BOARD, TURN, END. The brain can ignore INFO commands which are not needed. The reply for every other command is UNKNOWN (Due to backward compatibility and possibility to extend the protocol).

## Brain's name and temporary files

via files (old method)

```
Example:
pbrain-swine.exe
pbrain-pisq5.exe
```

Working directory is set by the manager. It doesn't have to be the directory where the brain's executable file is saved. The brain must specify full path to all data files which it uses. It can obtain the path from function GetModuleFileName or it can look at the beginning of its command line which can be discovered from the main function parameters. The manager must put name of brain's exe file on the command line in such a form so that the brain can open the file.

The brain can create a folder in the current directory to store its temporary files. The name of the folder must be the same as the name of the brain. The maximal allowed size of the folder will be announced on Gomocup web page (it is now 20MB). The manager can delete all temporary files when the manager exits or after a tournament finished. Command INFO folder is used to determine folder where persistent files can be saved.

## Mandatory commands

### START [size]

```
Example:
 The manager sends:
  START 20
 The brain answers:
  OK - everything is good
  ERROR message - unsupported size or other error
```

### TURN [X],[Y]

```
Expected answer:
 two comma-separated numbers - coordinates of the brain's move

Example:
 The manager sends:
  TURN 10,10
 The brain answers:
  11,10
```

### BEGIN

```
Expected answer:
 two numbers separated by comma - coordinates of the brain's move

Example:
 The manager sends:
  BEGIN
 The brain answers:
  10,10
```

### BOARD

After this command the data forming the playing field are send. Every line is in the form:

```
 [X],[Y],[field]
```

If game rule is renju, then the manager must send these lines in the same order as moves were made. If game rule is Gomoku, then the manager may send moves in any order and the brain must somehow cope with it. Data are ended by DONE command. Then the brain is expected to answer such as to TURN or BEGIN command.

```
Example:
 The manager sends:
  BOARD
  10,10,1
  10,11,2
  11,11,1
  9,10,2
  DONE
 The brain answers:
  9,9
```

### INFO [key] [value]

```
 The key can be:
timeout_turn  - time limit for each move (milliseconds, 0=play as fast as possible)
timeout_match - time limit of a whole match (milliseconds, 0=no limit)
max_memory    - memory limit (bytes, 0=no limit)
time_left     - remaining time limit of a whole match (milliseconds)
game_type     - 0=opponent is human, 1=opponent is brain, 2=tournament, 3=network tournament
rule          - bitmask or sum of 1=exactly five in a row win, 2=continuous game, 4=renju, 8=caro
evaluate      - coordinates X,Y representing current position of the mouse cursor
folder        - folder for persistent files
```

Time for a match is measured from creating a process to the end of a game (but not during opponent's turn). Time for a turn includes processing of all commands except initialization (commands START, RECTSTART, RESTART). Turn limit equal to zero means that the brain should play as fast as possible (eg count only a static evaluation and don't search possible moves).

INFO folder is used to determine a folder for files that are permanent. Because this folder is common for all brains and maybe other applications, the brain must create its own subfolder which name must be the same as the name of the brain. If the manager does not send INFO folder, then the brain cannot store permanent files.

Only debug versions should respond to INFO evaluate. For example, it can print evaluation of the square to some window. It cannot be written to the standard output. Release versions should just ignore INFO evaluate.

How should the brain behave when obtains unknown INFO command ?
 - Ignore it, it is probably not important. If it was important, it is not in an INFO command form.

How should behave the brain obtaining the unachievable INFO command?
 (for example too small memory limit)
 - The brain should wait with the output of the problem until the manager sends the first command not having an INFO form (TURN, BOARD or BEGIN). The manager does not read messages from the brain when sending INFO command.

```
Example:
 INFO timeout_match 300000
 INFO timeout_turn 10000
 INFO max_memory 83886080

 Expected answer: none
```

### END

```
 Expected answer: none
 The brain should delete its temporary files.
```

### ABOUT

```
Example:
 The manager sends:
  ABOUT
 The brain answers:
  name="SomeBrain", version="1.0", author="Nymand", country="USA"
```

## Optional commands

### RECTSTART [width],[height]

```
Example:
 The manager sends:
  RECTSTART 30,20
 The brain answers:
  OK - parameters are good
  ERROR message - rectangular board is not supported or other error
```

### RESTART

```
Example:
 The manager sends:
  RESTART
 The brain answers:
  OK
```

### TAKEBACK [X],[Y]

```
Example:
 The manager sends:
  TAKEBACK 9,10
 The brain answers:
  OK
```

### PLAY [X],[Y]

```
Example:
 The brain has sent:
  SUGGEST 10,10
 The manager sends:
  PLAY 12,10
 The brain moves onto 12,10 and answers:
  12,10
```

### SWAP2BOARD

Swap2

- Case 1. The manager asks for the first three stones. The manager sends: SWAP2BOARD DONE The AI answers: 7,7 8,7 9,9
- Case 2. The manager sends the coordinates of the first three stones and asks for the choice of options. The manager sends: SWAP2BOARD 7,7 8,7 9,9 DONE The AI answers: SWAP - if the AI decides to swap (option 1) 8,8 - output the coordinate of the 4th move if the AI decides to stay with its color (option 2) 8,8 8,6 - output the coordinates of the 4th and 5th stones if the AI decides to put two stones and let the opponent choose the color (option 3)
- Case 3. The manager sends the coordinates of the first five stones and asks for the choice of options. The manager sends: SWAP2BOARD 7,7 8,7 9,9 8,8 8,6 DONE The AI answers: SWAP - if the AI decides to swap (option 1) 6,8 - output the coordinate of the 6th move if the AI decides to stay with its color (option 2)
- After the opening stage, the stones on the board will be treated as an opening for a standard match. For example, following the above example in Case 3, assuming the AI chooses option 2, the manager will send the following messages to the other AI: BOARD 7,7,1 8,7,2 9,9,1 8,8,2 8,6,1 6,8,2 DONE
- As another example, following Case 2's example, assuming the AI chooses option 1, the manager will send the following messages to the other AI: BOARD 7,7,2 8,7,1 9,9,2 DONE

## Commands that are sent by the brain

### UNKNOWN [error message]

### ERROR [error message]

### MESSAGE [message]

It is recommended to send only English messages. If the brain chooses another language, it should detect code page that is used on the PC (Win32 function GetACP()) and should not send messages that cannot be displayed in that code page.

### DEBUG [message]

```
Example:
 The manager sends:
  TURN 10,15
 The brain answers:
  DEBUG The most promising move now is [10,14] alfa=10025 beta=8641
  DEBUG The most promising move now is [11,14] alfa=10125 beta=8641
  MESSAGE I will be the winner
  10,16
```

### SUGGEST [X],[Y]

## Version history

- 2023-03-20
INFO rule 8 - Caro
- 2022-12-20
SWAP2BOARD command
- 2016-02-07
coordinates in BOARD command must be in correct order if rule is renju
both 64-bit and 32-bit exe can be in ZIP
- 2016-02-02
INFO rule 4 - renju
- 2006-03-11
INFO rule 2 - option continuous game, also added value 3 to the BOARD command
- 2005-12-19
INFO folder - for permanent files
ABOUT - format changed to keyword="value"
INFO timeout_turn 0 means as fast as possible
- 2005-06-26
TAKEBACK - used for undo
- 2005-06-03
INFO rule - option exactly five in a row
- 2005-05-19
INFO game_type
- 2005-04-21
RECTSTART - rectangular board
RESTART - clears the board
BOARD - it is mandatory command