import {
    Badge,
    Box, Button, Editable, EditableInput, EditablePreview,
    HStack,
    IconButton, Select, Spacer, Tab, TabList, TabPanel, TabPanels,
    Tabs
} from "@chakra-ui/react";
import { React, useContext, useEffect, useState } from "react";
import { BsTrash, MdVerticalAlignTop } from "react-icons/all";
import T from "../core/T";
import Utils from '../core/utils';
import Server from '../server';
import UserContext from '../UserContext';
import ConfirmChangeOwner from './ConfirmChangeOwner';
import ConfirmDelete from './ConfirmDelete';
import Progress from './Progress';
import Properties from './Properties';
import TaskEditor from './TaskEditor';
import TaskViewer from './TaskViewer';
import Files from './Files';



function Task(props) {
    const { project, info } = useContext(UserContext);
    const { board, name, modTime } = props.info;
    const { compact, boards, users, searchKeys } = props;
    const [task, setTask] = useState(null)
    const [progress, setProgress] = useState('')
    const [openConfirmDelete, setOpenConfirmDelete] = useState(false)
    const [candidateOwner, setCandidateOwner] = useState(null)

    function getTags(task) {
        function extractTags(text) {
            const tags = []
            if (text) {
                const re = /(#\w+)/g
                while (true) {
                    const m = re.exec(text);
                    if (m) { tags.push(m[1]) } else break
                }
            }
            return tags
        }

        let tags = extractTags(task.description)
        for (const value of Object.values(task.properties)) {
            tags = [...tags, ...extractTags(value)]
        }
        for (const part of Object.values(task.parts)) {
            tags = [...tags, ...extractTags(part)]
        }
        return tags
    }

    function updateProgress(task) {
        const progress = task && task.parts && task.parts.length ?
            `${Math.round(100 * task.parts.filter(p => p.done).length / task.parts.length)}%`
            : '-'
        setProgress(progress)
        return task
    }

    function getTask() {
        Server.getTask(project, board, name)
            .then(updateProgress)
            .then(setTask)
    }
    useEffect(getTask, [])

    function touchTask() {
        Server.touchTask(project, board, name)
            .then(_ => props.onBoardChanged && props.onBoardChanged())
    }

    function saveTask(task) {
        Server.setTaskLater(project, board, name, task)
    }

    function renameTask(title) {
        Server.moveTask(project, board, name, board, title)
            .then(_ => props.onBoardChanged && props.onBoardChanged())
    }

    function deleteTask() {
        Server.deleteTask(project, board, name)
            .then(_ => props.onBoardChanged && props.onBoardChanged())
            .then(_ => setOpenConfirmDelete(false))
    }

    function changeOwner(evt) {
        const newOwner = evt && evt.target && evt.target.value;
        if (newOwner) {
            if (owner == info.systemUser) {
                task.properties['Owner'] = `@${newOwner}`;
                saveTask(task)
                setTask({ ...task })
            } else {
                setCandidateOwner(newOwner);
            }
        }
    }

    function confirmCandidateOwner() {
        task.properties['Owner'] = `@${candidateOwner}`;
        saveTask(task)
        setTask({ ...task })
        setCandidateOwner(null);
    }

    function onBoardChanged(evt) {
        const newBoard = evt && evt.target && evt.target.value;
        newBoard && Server.moveTask(project, board, name, newBoard)
        props.onBoardChanged && props.onBoardChanged()
    }

    if (!task) return null;

    const owner = task && task.properties && task.properties['Owner']
        && task.properties['Owner'].substring(1)
    const readOnly = owner != info.systemUser

    const [id, title] = name && name.split(/\.(.+)/) || ['', 'Something went wrong']
    const userList = users && users.map(u => <option key={u} value={u}>
        {u}
    </option>)
    const boardList = boards && boards.map(b => <option key={b} value={b}>
        {b}
    </option>)

    const mtime = `Last modified: ${Utils.getFriendlyDate(modTime)}`
    const tags = task ? getTags(task).map(tag => <Badge key={tag} colorScheme="purple">
        {tag}
    </Badge>) : null;
    const header = task && <HStack spacing={3}>
        <label>{id}.</label>
        <Editable defaultValue={title} borderWidth="1px" minW="300px"
            borderColor="blue" onSubmit={title => renameTask(title)}>
            <EditablePreview />
            <EditableInput />
        </Editable>
        <Spacer />
        {compact ? <HStack h="2em" spacing={2}>{tags}</HStack> : null}
        <Button size="sm" title={mtime}><MdVerticalAlignTop onClick={touchTask} /></Button>
        <span title="Task Progress">{progress}</span>
        <Select value={board} title="Assign the Board" w="10em" onChange={onBoardChanged}>
            {boardList}
        </Select>
        <Select value={owner} title="Assign the Owner" w="10em" onChange={changeOwner}>
            {userList}
        </Select>
        <IconButton title="Delete the task" onClick={_ => setOpenConfirmDelete(true)}>
            <BsTrash />
        </IconButton>
    </HStack>

    function onChange(index) {
        if (index == 0) {
            setTask({ ...task, description: task.description });
        }
    }

    const body = task && !compact ? <HStack spacing={3}>
        <ConfirmChangeOwner owner={owner} candidateOwner={candidateOwner}
            setCandidateOwner={setCandidateOwner} onConfirm={confirmCandidateOwner} />
        <ConfirmDelete isOpen={openConfirmDelete} setIsOpen={setOpenConfirmDelete}
            onConfirm={deleteTask} />
        <Tabs w="100%" onChange={onChange}>
            <TabList>
                <Tab key="view"><T>view</T></Tab>
                <Tab key="edit"><T>edit</T></Tab>
                <Tab key="properties"><T>properties</T></Tab>
                <Tab key="progress"><T>progress</T></Tab>
                <Tab key="files"><T>files</T></Tab>
                <Spacer key="spacer" />
                <HStack h="2em" spacing={2} key="tags">{tags}</HStack>
            </TabList>

            <TabPanels>
                <TabPanel key="view" padding={0}>
                    <TaskViewer task={task} saveTask={saveTask} searchKeys={searchKeys} />
                </TabPanel>
                <TabPanel key="edit" padding={0}>
                    <TaskEditor task={task} saveTask={saveTask} users={users}
                        readOnly={readOnly} />
                </TabPanel>
                <TabPanel key="properties" >
                    <Properties task={task} saveTask={saveTask} users={users} readOnly={readOnly} />
                </TabPanel>
                <TabPanel key="progress" >
                    <Progress task={task} readOnly={readOnly}
                        saveTask={task => {
                            saveTask(task);
                            updateProgress(task);
                        }} />
                </TabPanel>
                <TabPanel>
                    <Files task={task} saveTask={saveTask} readOnly={readOnly}/>
                </TabPanel>
            </TabPanels>
        </Tabs>
    </HStack> : ''

    return task ? <Box p={1} w="100%" borderWidth="3px" overflow="hidden">
        {header}
        {body}
    </Box> : ''

}

export default Task;