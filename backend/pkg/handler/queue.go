package handler

import (
	"net/http"

	"soneph-backend/pkg/store"

	"github.com/gin-gonic/gin"
) // GetQueue retourne la file de lecture persistée (ordre + index courant).
// Depuis M3, la file vit côté serveur : visible depuis un second navigateur.

func (a *API) GetQueue(c *gin.Context) {
	q, err := a.st.GetPlayerQueue()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, q)
} // SaveQueue persiste la file de lecture (ordre + index).

func (a *API) SaveQueue(c *gin.Context) {
	var req store.PlayerQueue
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "'queue' (liste de chemins) et 'index' (nombre) attendus"})
		return
	}
	if err := a.st.SetPlayerQueue(req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "File enregistrée"})
}
