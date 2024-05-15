/*--------------------------------------------------------------------------
 * Copyright (c) 2019-2021, Postgres.ai, Nikolay Samokhvalov nik@postgres.ai
 * All Rights Reserved. Proprietary and confidential.
 * Unauthorized copying of this file, via any medium is strictly prohibited
 *--------------------------------------------------------------------------
 */

import React from "react";
import { Link } from "react-router-dom";
import { useParams } from "react-router";
import cn from "classnames";
import { makeStyles, Theme } from "@material-ui/core";
import Drawer from '@material-ui/core/Drawer';
import List from "@material-ui/core/List";
import Divider from "@material-ui/core/Divider";
import ListSubheader from '@material-ui/core/ListSubheader';
import Box from "@mui/material/Box";
import { Spinner } from "@postgres.ai/shared/components/Spinner";
import { HeaderButtons, HeaderButtonsProps } from "../HeaderButtons/HeaderButtons";
import { BotMessage } from "../../../types/api/entities/bot";


const useStyles = makeStyles<Theme, ChatsListProps>((theme) => ({
    drawerPaper: {
      width: 240,
      //TODO: Fix magic numbers
      height: props => props.isDemoOrg ? 'calc(100vh - 122px)' : 'calc(100vh - 90px)',
      marginTop: props => props.isDemoOrg ? 72 : 40,
      [theme.breakpoints.down('sm')]: {
        height: '100vh!important',
        marginTop: '0!important',
        width: 260,
        zIndex: 9999
      },
      '& > ul': {
        display: 'flex',
        flexDirection: 'column',
        '@supports (scrollbar-gutter: stable)': {
          scrollbarGutter: 'stable',
          paddingRight: 0,
          overflow: 'hidden',
        },
        '&:hover': {
          overflow: 'auto'
        },
        [theme.breakpoints.down('sm')]: {
          paddingBottom: 120
        }
      }
    },
    listPadding: {
      paddingTop: 0
    },
    listSubheaderRoot: {
      background: 'white',
    },
    listItemLink: {
      fontFamily: '"Roboto", "Helvetica", "Arial", sans-serif',
      fontStyle: 'normal',
      fontWeight: 'normal',
      fontSize: '0.875rem',
      lineHeight: '1rem',
      color: '#000000',
      width: '100%',
      textOverflow: 'ellipsis',
      overflow: 'hidden',
      padding: '0.75rem 1rem',
      whiteSpace: 'nowrap',
      textDecoration: "none",
      flex: '0 0 2.5rem',
      '&:hover': {
        background: 'rgba(0, 0, 0, 0.04)'
      }
    },
    listItemLinkActive: {
      background: 'rgba(0, 0, 0, 0.04)'
    },
    loader: {
      width: '100%',
      height: '100%',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center'
    }
  })
);

type ChatsListProps = {
  isOpen: boolean;
  onCreateNewChat: () => void;
  onClose: () => void;
  isDemoOrg: boolean;
  loading: boolean;
  chatsList: BotMessage[] | null;
  onLinkClick?: (targetThreadId: string) => void;
  permalinkId?: string
} & HeaderButtonsProps

export const ChatsList = (props: ChatsListProps) => {
  const {
    isOpen,
    onCreateNewChat,
    onClose,
    chatsList,
    loading,
    currentVisibility,
    withChatVisibilityButton,
    onChatVisibilityClick,
    onLinkClick,
    permalinkId
  } = props;
  const classes = useStyles(props);
  const params = useParams<{ org?: string, threadId?: string }>();

  const linkBuilder = (msgId: string) => {
    if (params.org) {
      return `/${params.org}/bot/${msgId}`
    } else {
      return `/bot/${msgId}`
    }
  }

  const handleClick = (threadId: string) => {
    if (onLinkClick) {
      onLinkClick(threadId)
    }
  }

  const loader = (
    <Box className={classes.loader}>
      <Spinner/>
    </Box>
  )

  const list = (
    <List
      classes={{padding: classes.listPadding}}
    >
      <ListSubheader
        classes={{root: classes.listSubheaderRoot}}
      >
        <HeaderButtons
          onClose={onClose}
          onCreateNewChat={onCreateNewChat}
          isOpen={isOpen}
          currentVisibility={currentVisibility}
          withChatVisibilityButton={withChatVisibilityButton}
          onChatVisibilityClick={onChatVisibilityClick}
          permalinkId={permalinkId}
        />
        <Divider/>
      </ListSubheader>

      {chatsList && chatsList.map((item) => {
        const isActive = item.id === params.threadId
        const link = linkBuilder(item.id)
        return (
          <Link
            to={link}
            key={item.id}
            className={cn(classes.listItemLink, {[classes.listItemLinkActive]: isActive})}
            id={item.id}
            onClick={() => handleClick(item.id)}
          >
            {item.content}
          </Link>
        )
      })
      }
    </List>
  )

  return (
    <Drawer
      variant={'persistent'}
      anchor="right"
      BackdropProps={{ invisible: true }}
      elevation={1}
      open={isOpen}
      classes={{
        paper: classes.drawerPaper
      }}
    >
      {loading ? loader : list}
    </Drawer>
  )
}