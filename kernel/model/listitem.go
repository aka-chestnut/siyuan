// SiYuan - Refactor your thinking
// Copyright (c) 2020-present, b3log.org
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package model

import (
	"path"
	"strings"

	"github.com/88250/lute/ast"
	"github.com/88250/lute/parse"
	"github.com/siyuan-note/siyuan/kernel/sql"
	"github.com/siyuan-note/siyuan/kernel/treenode"
	"github.com/siyuan-note/siyuan/kernel/util"
)

func ListItem2Doc(srcListItemID, targetBoxID, targetPath, previousPath string) (srcRootBlockID, newTargetPath string, err error) {
	FlushTxQueue()

	srcTree, _ := LoadTreeByBlockID(srcListItemID)
	if nil == srcTree {
		err = ErrBlockNotFound
		return
	}
	srcRootBlockID = srcTree.Root.ID

	listItemNode := treenode.GetNodeInTree(srcTree, srcListItemID)
	if nil == listItemNode {
		err = ErrBlockNotFound
		return
	}

	box := Conf.Box(targetBoxID)
	listItemText := sql.GetContainerText(listItemNode)
	listItemText = util.FilterFileName(listItemText)

	moveToRoot := "/" == targetPath
	toHP := path.Join("/", listItemText)
	toFolder := "/"

	if "" != previousPath {
		previousDoc := treenode.GetBlockTreeRootByPath(targetBoxID, previousPath)
		if nil == previousDoc {
			err = ErrBlockNotFound
			return
		}
		parentPath := path.Dir(previousPath)
		if "/" != parentPath {
			parentPath = strings.TrimSuffix(parentPath, "/") + ".sy"
			parentDoc := treenode.GetBlockTreeRootByPath(targetBoxID, parentPath)
			if nil == parentDoc {
				err = ErrBlockNotFound
				return
			}
			toHP = path.Join(parentDoc.HPath, listItemText)
			toFolder = path.Join(path.Dir(parentPath), parentDoc.ID)
		}
	} else {
		if !moveToRoot {
			parentDoc := treenode.GetBlockTreeRootByPath(targetBoxID, targetPath)
			if nil == parentDoc {
				err = ErrBlockNotFound
				return
			}
			toHP = path.Join(parentDoc.HPath, listItemText)
			toFolder = path.Join(path.Dir(targetPath), parentDoc.ID)
		}
	}

	newTargetPath = path.Join(toFolder, srcListItemID+".sy")
	if !box.Exist(toFolder) {
		if err = box.MkdirAll(toFolder); err != nil {
			return
		}
	}

	var children []*ast.Node
	for c := listItemNode.FirstChild; nil != c; c = c.Next {
		if c.IsMarker() {
			continue
		}
		children = append(children, c)
	}
	if 1 > len(children) {
		newNode := treenode.NewParagraph("")
		children = append(children, newNode)
	}

	luteEngine := util.NewLute()
	newTree := &parse.Tree{Root: &ast.Node{Type: ast.NodeDocument, ID: srcListItemID}, Context: &parse.Context{ParseOption: luteEngine.ParseOptions}}
	for _, c := range children {
		newTree.Root.AppendChild(c)
	}
	newTree.ID = srcListItemID
	newTree.Path = newTargetPath
	newTree.HPath = toHP
	listItemNode.SetIALAttr("type", "doc")
	listItemNode.SetIALAttr("id", srcListItemID)
	listItemNode.SetIALAttr("title", listItemText)
	listItemNode.RemoveIALAttr("fold")
	newTree.Root.KramdownIAL = listItemNode.KramdownIAL
	srcLiParent := listItemNode.Parent
	listItemNode.Unlink()
	if nil != srcLiParent && nil == srcLiParent.FirstChild {
		srcLiParent.Unlink()
	}
	srcTree.Root.SetIALAttr("updated", util.CurrentTimeSecondsStr())
	if nil == srcTree.Root.FirstChild {
		srcTree.Root.AppendChild(treenode.NewParagraph(""))
	}
	treenode.RemoveBlockTreesByRootID(srcTree.ID)
	if err = indexWriteTreeUpsertQueue(srcTree); err != nil {
		return "", "", err
	}

	newTree.Box, newTree.Path = targetBoxID, newTargetPath
	newTree.Root.SetIALAttr("updated", util.CurrentTimeSecondsStr())
	newTree.Root.Spec = "1"
	if "" != previousPath {
		box.addSort(previousPath, newTree.ID)
	} else {
		box.addMinSort(path.Dir(newTargetPath), newTree.ID)
	}
	if err = indexWriteTreeUpsertQueue(newTree); err != nil {
		return "", "", err
	}
	IncSync()
	go func() {
		RefreshBacklink(srcTree.ID)
		RefreshBacklink(newTree.ID)
		ResetVirtualBlockRefCache()
	}()
	return
}
