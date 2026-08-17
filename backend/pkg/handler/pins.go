package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
) // GetPins retourne les épingles (artistes/albums/playlists) de la base —
// source de vérité depuis M3 : plus rien en localStorage.

func (a *API) GetPins(c *gin.Context) {
	pins, err := a.st.ListPins()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"pins": pins})
} // AddPin épingle une entrée (kind: artist|album|playlist, value: nom/id).

func (a *API) AddPin(c *gin.Context) {
	var req struct {
		Kind  string `json:"kind"`
		Value string `json:"value"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Value) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "'kind' et 'value' sont requis"})
		return
	}
	if err := a.st.AddPin(strings.TrimSpace(req.Kind), strings.TrimSpace(req.Value)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Épingle ajoutée"})
} // RemovePin retire une épingle (kind + value en query).

func (a *API) RemovePin(c *gin.Context) {
	kind := c.Query("kind")
	value := c.Query("value")
	if kind == "" || value == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query params 'kind' et 'value' sont requis"})
		return
	}
	if err := a.st.RemovePin(kind, value); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Épingle retirée"})
}
