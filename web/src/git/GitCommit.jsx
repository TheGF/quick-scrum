import {
    Accordion,
    AccordionButton,
    AccordionIcon, AccordionItem,
    AccordionPanel, Box, Button, Center, Spacer,
    HStack, List, ListIcon, ListItem, StackDivider,
    Textarea, VStack, Flex, Input, Text, Tbody, Table, Tr, Td,
} from "@chakra-ui/react";
import { React, useContext, useEffect, useState } from "react";
import { BiCheckCircle, BiCircle } from "react-icons/bi";
import Server from '../server';
import UserContext from '../UserContext';

function GitCommit(props) {
    const { project, info } = useContext(UserContext)
    const { gitStatus, gitMessage, onCommit } = props
    const [commitOutput, setCommitOutput] = useState(null)
    const [commitInProgress, setCommitInProgress] = useState(false)
    const commitInfo = {
        user: info.loginUser,
        header: gitMessage.header,
        body: gitMessage.body,
        files: gitStatus && [...gitStatus.ashFiles, ...gitStatus.stagedFiles] || [],
    }

    function commit() {
        setCommitInProgress(true)
        Server.postGitCommit(project, commitInfo)
            .then(setCommitOutput)
            .then(_ => setCommitInProgress(false))
            .then(onCommit)
    }

    const summary = [
        ['User', commitInfo.user],
        ['Header', commitInfo.header],
        ['Tasks', Object.keys(commitInfo.body).join(' ')],
        ['Staged Files', commitInfo.files.join(' ')],
    ]
    const table = summary.map(r => <Tr>
        <Td key={r[0]}>{r[0]}</Td>
        <Td>{r[1]}</Td>
    </Tr>)

    return commitOutput ?
        <VStack>
            <Text fontSize="md" color="green">
                Commit was successful
            </Text>
            <Textarea
                value={commitOutput}
                size="md"
                resize="Vertical"
                rows="10"
            />
        </VStack> :
        <VStack>
            <VStack textAlign="left">
                <Table>
                    <Tbody>
                        {table}
                    </Tbody>
                </Table>
            </VStack>
            <HStack spacing={5}>
                <Button size="lg" colorScheme="blue" isLoading={commitInProgress}
                    onClick={commit}>
                    Commit
            </Button>
            </HStack>
        </VStack>

}
export default GitCommit;